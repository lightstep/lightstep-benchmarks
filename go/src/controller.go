package main

import (
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"benchlib"

	"github.com/GaryBoone/GoStats/stats"
	"github.com/golang/glog"
	lst "github.com/lightstep/lightstep-tracer-go/lightstep_thrift"
	"github.com/lightstep/lightstep-tracer-go/thrift_0_9_2/lib/go/thrift"
)

const (
	// collectorBinaryPath is the path of the Thrift collector service
	collectorBinaryPath = "/_rpc/v1/reports/binary"
	// collectorJSONPath is the path of the pure-JSON collector service
	collectorJSONPath = "/api/v0/reports"

	// testIteration is used for initial estimates and calibration.
	testIteration = 1000

	// maxConcurrency is the limit of concurrency testing
	maxConcurrency = 3

	// testTolerance is used for a sanity checks.
	// TODO this is too lax, but needed for testing on busy laptops.
	testTolerance = 0.1

	nanosPerSecond = 1e9
)

var (
	// testTimeSlice is a small duration used to set a minimum
	// reasonable execution time during calibration.
	testTimeSlice = 10 * time.Millisecond

	// client is a list of client programs for the benchmark
	clients = []benchClient{
		{"nodejs", []string{"node", "./jsclient.js"}},
		{"python", []string{"./pyclient.py"}},
		{"golang", []string{"./goclient"}},
	}

	// requestCh is used to serialize HTTP requests
	requestCh = make(chan sreq)

	headless = flag.Bool("headless", true,
		"If true, this process will the run the test clients "+
			" itself. Otherwise, tests may be run manually by setting "+
			"this false.")
)

type saturationTest struct {
	trace   bool
	seconds float64
	qps     float64
	load    float64
	lognum  int64
	logsize int64
}

type sreq struct {
	w      http.ResponseWriter
	r      *http.Request
	doneCh chan struct{}
}

type benchService struct {
	processor        *lst.ReportingServiceProcessor
	processorFactory thrift.TProcessorFactory
	protocolFactory  thrift.TProtocolFactory
	controlCh        chan *benchlib.Control
	resultCh         chan *benchlib.Result

	// outstanding request state
	controlling bool
	before      benchlib.Timing

	// current collects results for the current test
	current *benchStats
}

type benchStats struct {
	// Note: using float64 instead of time.Duration since these
	// are very small units of time.

	// Note: []zeroCost, roundCost, and workCost are computed from
	// the slope of their respective line, without considering the
	// intercept. The intercept should be close to 0, but it is not
	// being verified.

	// The cost of doing zero repetitions, indexed by concurrency.
	zeroCost []benchlib.Timing

	// The cost of a round w/ no working, no sleeping, no tracing.
	roundCost benchlib.Timing

	// The cost of a single unit of work.
	workCost benchlib.Timing

	// Cost of tracing a span that does no work
	spanCost benchlib.Timing

	// Fixed cost of sleeping (per amortized sleep interval)
	sleepCost benchlib.Timing

	spansReceived int64
	bytesReceived int64
}

type benchClient struct {
	Name string
	Args []string
}

func newBenchStats() *benchStats {
	return &benchStats{
		zeroCost: make([]benchlib.Timing, maxConcurrency+1, maxConcurrency+1),
	}
}

func serializeHTTP(w http.ResponseWriter, r *http.Request) {
	doneCh := make(chan struct{})
	requestCh <- sreq{w, r, doneCh}
	<-doneCh
}

func fakeReportResponse() *lst.ReportResponse {
	nowMicros := time.Now().UnixNano() / 1000
	return &lst.ReportResponse{Timing: &lst.Timing{&nowMicros, &nowMicros}}
}

// Report is a Thrift Collector method.
func (s *benchService) Report(auth *lst.Auth, request *lst.ReportRequest) (
	r *lst.ReportResponse, err error) {
	s.current.spansReceived += int64(len(request.SpanRecords))
	return fakeReportResponse(), nil
}

// BytesReceived is called from the HTTP layer before Thrift
// processing, recording inbound byte count.
func (s *benchService) BytesReceived(num int64) {
	s.current.bytesReceived += num
}

// estimateZeroCosts measures the cost of doing nothing.
func (s *benchService) estimateZeroCosts() {
	for c := 1; c <= maxConcurrency; c++ {
		var st benchlib.TimingStats
		for j := 0; j < testIteration; j++ {
			tm := s.run(&benchlib.Control{
				Concurrent: c,
			})
			st.Update(tm.Adjusted)
		}
		glog.V(1).Infoln("Cost of zero repeats", st, "conc", c)
		s.current.zeroCost[c] = st.Mean()
	}
}

// measureSpanCost runs a closed loop creating a certain
// number of spans as quickly as possible and reporting
// the timing.
func (s *benchService) measureSpanCost() {
	s.current.spanCost = s.measureTestLoop(true)
	glog.Info("Span creation cost: ", s.current.spanCost)
}

// estimateRoundCost runs a untraced loop doing no work to establish
// the baseline cost of a repetition.
func (s *benchService) estimateRoundCost() {
	s.current.roundCost = s.measureTestLoop(false)
	glog.V(1).Infoln("Cost of single round", s.current.roundCost)
}

// estimateWorkCosts measures the cost of the work function.
func (s *benchService) estimateWorkCost() {
	// The work function is assumed to be fast. Find a multiplier
	// that results in working at least testTimeSlice.
	multiplier := int64(1000)
	for {
		tm := s.run(&benchlib.Control{
			Concurrent: 1,
			Work:       multiplier,
			Repeat:     1,
		})
		if tm.Adjusted.Wall.Seconds() > testTimeSlice.Seconds() {
			break
		}
		multiplier *= 10
	}

	// Compute data points for factors of the multiplier.
	data := benchlib.Timings{}
	for iter := 1; iter <= 10; iter++ {
		repeat := int64(iter) * multiplier
		for j := 0; j < 10; j++ {
			tm := s.run(&benchlib.Control{
				Concurrent: 1,
				Work:       repeat,
				Repeat:     1,
			})

			data.Update(float64(repeat), tm.Adjusted)
		}
	}

	reg := data.LinearRegression()
	s.current.workCost = reg.Slope()
	glog.V(1).Infof("Work cost %.2gs/unit", s.current.workCost.Wall)
}

func (s *benchService) sanityCheckWork() {
	runfor := time.Second.Seconds() * 10
	conc := 1
	var st benchlib.TimingStats
	for i := 0; i < 10; i++ {
		work := int64(runfor / s.current.workCost.Wall.Seconds())
		tm := s.run(&benchlib.Control{
			Concurrent: conc,
			Work:       work,
			Repeat:     1,
		})
		st.Update(tm.Adjusted)
	}
	glog.V(1).Infoln("Check work timing", st, "expected", runfor)

	absRatio := math.Abs((st.Wall.Mean() - runfor) / runfor)
	if absRatio > testTolerance {
		glog.Fatal("CPU work not well calibrated (or insufficient CPU) @ concurrency ",
			conc, ": measured ", st.Mean(), " expected ", runfor,
			" off by ", absRatio*100.0, "%")
	}
}

func (s *benchService) measureTestLoop(trace bool) benchlib.Timing {
	multiplier := int64(1000)
	for {
		tm := s.run(&benchlib.Control{
			Concurrent: 1,
			Work:       0,
			Repeat:     multiplier,
			Trace:      trace,
			NoFlush:    true,
		})
		if tm.Adjusted.Wall.Seconds() > testTimeSlice.Seconds() {
			break
		}
		multiplier *= 10
	}
	multiplier /= 10
	var data benchlib.Timings
	for i := 1; i <= 10; i++ {
		rounds := multiplier * int64(i)
		for j := 0; j < 10; j++ {
			tm := s.run(&benchlib.Control{
				Concurrent: 1,
				Work:       0,
				Repeat:     rounds,
				Trace:      trace,
				NoFlush:    true,
			})
			data.Update(float64(rounds), tm.Adjusted)
		}
	}
	// This test didn't flush, flush now.
	s.flush()
	reg := data.LinearRegression()
	return reg.Slope()
}

func (s *benchService) estimateSleepCost() {
	latency := benchlib.DefaultSleepInterval
	duration := latency / 2

	tm := s.run(&benchlib.Control{
		Concurrent: 1,
		Work:       int64(duration.Seconds() / s.current.workCost.Wall.Seconds()),
		Sleep:      duration,
		Repeat:     int64(10 * time.Second / latency),
	})
	if tm.Sleeps.Count() == 0 {
		// No calibration.
		return
	}

	diff := tm.Sleeps.Mean() - latency.Seconds()
	s.current.sleepCost.Wall = benchlib.Time(diff)
	glog.V(1).Info("Sleep calibration past-due average ", s.current.sleepCost)
}

func (s *benchService) measureSpanSaturation(opts saturationTest) benchlib.TimingStats {
	workTime := opts.load / opts.qps
	sleepTime := (1 - opts.load) / opts.qps

	// To maintain the target load and measure only deferred work
	// while tracing, subtract the measured span cost from each
	// sleep.  If the sleep is too little, return 0 to indicate
	// saturation.
	if opts.trace {
		if sleepTime < s.current.spanCost.Wall.Seconds() {
			glog.Info("QPS span cost adjustment too small, skipping sleep")
			sleepTime = 0
		} else {
			sleepTime -= s.current.spanCost.Wall.Seconds()
		}
	}

	var ss benchlib.TimingStats
	var spans stats.Stats
	var bytes stats.Stats
	total := opts.seconds * opts.qps
	for i := 0; i < 5; i++ {
		sbefore := s.current.spansReceived
		bbefore := s.current.bytesReceived
		tm := s.run(&benchlib.Control{
			Concurrent:  1,
			Work:        int64(workTime / s.current.workCost.Wall.Seconds()),
			Sleep:       time.Duration(sleepTime * nanosPerSecond),
			Repeat:      int64(total),
			Trace:       opts.trace,
			NumLogs:     opts.lognum,
			BytesPerLog: opts.logsize,
		})
		stotal := s.current.spansReceived - sbefore
		btotal := s.current.bytesReceived - bbefore

		glog.V(2).Info("Ran at ", 100*opts.load,
			"% for ",
			time.Duration(opts.seconds*nanosPerSecond),
			" saw ",
			stotal,
			" spans ",
			btotal,
			" bytes in ", tm.Adjusted)

		ss.Update(tm.Adjusted)
		spans.Update(float64(stotal))
		bytes.Update(float64(btotal))
	}
	tr := "untraced"
	if opts.trace {
		tr = "traced"
	}
	glog.Infof("Load %v@%3f%% %v == %.2f%% %.2fB/span (%s)",
		opts.qps, 100*opts.load, ss, (spans.Mean()/total)*100,
		(bytes.Mean() / spans.Mean()), tr)
	return ss
}

func (s *benchService) measureImpairment() {
	// Each test runs this long.
	const testTime = 30

	// Test will compute "impairment" measure for each QPS listed
	qpss := []float64{100 /*, 500, 1000*/}

	// Note: the timing includes a discount for the span creation cost.
	for _, qps := range qpss {
		for _, load := range []float64{
			//.1, .3, .5, .7, .9, .95, .96, .97, .98, .99, 0.995} {
			.05, .1, .15, .2, .25} {
			//.2} {
			s.measureSpanSaturation(saturationTest{
				trace:   true,
				seconds: testTime,
				qps:     qps,
				load:    load})
			s.measureSpanSaturation(saturationTest{
				trace:   false,
				seconds: testTime,
				qps:     qps,
				load:    load})
		}
	}
}

func (s *benchService) warmup() {
	s.run(&benchlib.Control{
		Concurrent: 1,
		Work:       10000,
		Repeat:     100,
	})
}

func (s *benchService) flush() {
	s.run(&benchlib.Control{
		Concurrent: 1,
		Work:       0,
		Repeat:     0,
		NoFlush:    false,
		Trace:      true,
	})
}

func (s *benchService) run(c *benchlib.Control) *benchlib.Result {
	if c.SleepInterval == 0 {
		c.SleepInterval = benchlib.DefaultSleepInterval
	}
	if c.SleepCorrection == 0 {
		c.SleepCorrection = s.current.sleepCost.Wall.Duration()
	}
	s.controlCh <- c
	r := <-s.resultCh
	r.Adjusted = r.Measured
	if s.current.zeroCost != nil {
		r.Adjusted.ReduceBy(s.current.zeroCost[c.Concurrent])
	}
	r.Adjusted.ReduceByFactor(s.current.roundCost, float64(c.Repeat))
	return r
}

func (s *benchService) runTests() {
	if !*headless {
		s.runTest(nil)
	} else {
		for _, bc := range clients {
			s.runTest(&bc)
		}
	}
	os.Exit(0)
}

func (s *benchService) runTest(bc *benchClient) {
	s.current = newBenchStats()

	if bc != nil {
		glog.Info("Testing ", bc.Name)
		ch := make(chan bool)

		defer func() {
			s.exitClient()
			<-ch
		}()

		go s.execClient(bc, ch)
	} else {
		glog.Info("Awaiting test client")
	}

	// Calibration
	s.warmup()
	s.estimateZeroCosts()
	s.estimateRoundCost()
	s.estimateWorkCost()
	s.estimateSleepCost()
	s.sanityCheckWork()

	// Measurement
	s.measureSpanCost()
	s.measureImpairment()
}

func (s *benchService) execClient(bc *benchClient, ch chan bool) {
	cmd := exec.Command(bc.Args[0], bc.Args[1:]...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Start(); err != nil {
		glog.Fatal("Could not start client: ", err)
	}
	if err := cmd.Wait(); err != nil {
		perr, ok := err.(*exec.ExitError)
		if !ok {
			glog.Fatal("Could not await client: ", err)
		}
		if !perr.Exited() {
			glog.Fatal("Client did not exit: ", err)
		}
		if !perr.Success() {
			glog.Fatal("Client failed: ", string(perr.Stderr))
		}
	}
	ch <- true
}

func (s *benchService) exitClient() {
	s.controlCh <- &benchlib.Control{Exit: true}
	s.controlling = false
}

// ServeControlHTTP returns a JSON control request to the client.
func (s *benchService) ServeControlHTTP(res http.ResponseWriter, req *http.Request) {
	if s.controlling {
		glog.Fatal("Out-of-phase control request", req.URL)
	}
	s.before = benchlib.GetChildUsage()
	s.controlling = true
	body, err := json.Marshal(<-s.controlCh)
	if err != nil {
		glog.Fatal("Marshal error: ", err)
	}
	res.Write(body)
}

// ServeResultHTTP records the client's result via a URL Query parameter "timing".
func (s *benchService) ServeResultHTTP(res http.ResponseWriter, req *http.Request) {
	if !s.controlling {
		glog.Fatal("Out-of-phase client result", req.URL)
	}
	usage := benchlib.GetChildUsage().Sub(s.before)
	// Note: it would be nice if there were a decoder to unmarshal
	// from URL query param into benchlib.Result, e.g., opposite of
	// https://godoc.org/github.com/google/go-querystring/query
	params, err := url.ParseQuery(req.URL.RawQuery)

	if err != nil {
		glog.Fatal("Error parsing URL params: ", req.URL.RawQuery)
	}
	s.controlling = false

	var sstat stats.Stats
	sleep_info := params.Get("s")
	if len(sleep_info) != 0 {
		for _, s := range strings.Split(sleep_info, ",") {
			if len(s) == 0 {
				continue
			}
			if snano, err := strconv.ParseUint(s, 10, 64); err != nil {
				glog.Fatal("Could not parse timing: ", s)
			} else {
				sstat.Update(float64(snano) / nanosPerSecond)
			}
		}
		glog.V(1).Info("Sleep timing: mean ", time.Duration(sstat.Mean()*nanosPerSecond),
			" stddev ", time.Duration(sstat.PopulationStandardDeviation()*nanosPerSecond))
	}
	s.resultCh <- &benchlib.Result{
		Measured: benchlib.Timing{
			Wall: benchlib.ParseTime(params.Get("timing")),
			User: usage.User,
			Sys:  usage.Sys,
		},
		Sleeps: sstat,
	}
}

// ServeJSONHTTP is more-or-less copied from crouton/cmd/collector/main.go
func (s *benchService) ServeJSONHTTP(res http.ResponseWriter, req *http.Request) {
	// Support the "Content-Encoding: gzip" if it's there
	var bodyReader io.ReadCloser
	switch req.Header.Get("Content-Encoding") {
	case "gzip":
		var err error
		bodyReader, err = gzip.NewReader(req.Body)
		if err != nil {
			http.Error(res, fmt.Sprintf("Could not decode gzipped content"),
				http.StatusBadRequest)
			return
		}
		defer bodyReader.Close()
	default:
		bodyReader = req.Body
	}

	body, err := ioutil.ReadAll(bodyReader)
	if err != nil {
		http.Error(res, "Unable to read body: "+err.Error(), http.StatusBadRequest)
		return
	}

	reportRequest := &lst.ReportRequest{}
	if err := json.Unmarshal(body, reportRequest); err != nil {
		http.Error(res, "Unable to decode body: "+err.Error(), http.StatusBadRequest)
		return
	}

	s.current.spansReceived += int64(len(reportRequest.SpanRecords))
	s.current.bytesReceived += int64(len(body))

	res.Header().Set("Content-Type", "application/json")
	if err = json.NewEncoder(res).Encode(fakeReportResponse()); err != nil {
		http.Error(res, "Unable to encode response: "+err.Error(), http.StatusBadRequest)
	}
}

func (s *benchService) ServeDefaultHTTP(res http.ResponseWriter, req *http.Request) {
	glog.Fatal("Unexpected HTTP request", req.URL)
}

func main() {
	flag.Parse()
	address := fmt.Sprintf(":%v", benchlib.ControllerPort)
	mux := http.NewServeMux()
	server := &http.Server{
		Addr:         address,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0 * time.Second,
		Handler:      http.HandlerFunc(serializeHTTP),
	}

	service := &benchService{}
	service.processor = lst.NewReportingServiceProcessor(service)
	service.resultCh = make(chan *benchlib.Result)
	service.controlCh = make(chan *benchlib.Control)

	go func() {
		for req := range requestCh {
			mux.ServeHTTP(req.w, req.r)
			close(req.doneCh)
		}
	}()

	tfactories := benchlib.ThriftFactories{
		thrift.NewTProcessorFactory(service.processor),
		thrift.NewTBinaryProtocolFactoryDefault(),
		service}

	mux.HandleFunc(collectorBinaryPath, tfactories.ServeThriftHTTP)
	mux.HandleFunc(collectorJSONPath, service.ServeJSONHTTP)
	mux.HandleFunc(benchlib.ControlPath, service.ServeControlHTTP)
	mux.HandleFunc(benchlib.ResultPath, service.ServeResultHTTP)
	mux.HandleFunc("/", service.ServeDefaultHTTP)

	go service.runTests()

	glog.Fatal(server.ListenAndServe())
}
