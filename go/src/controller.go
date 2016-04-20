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
	testTolerance = 0.02

	// For clients that require timing correction (i.e., NodeJS)
	timingTolerance = 0.015 // ~= 1 second per minute

	nanosPerSecond = 1e9

	minimumCalibrations = 5
)

var (
	// testTimeSlice is a small duration used to set a minimum
	// reasonable execution time during calibration.
	testTimeSlice = 10 * time.Millisecond

	// client is a list of client programs for the benchmark
	clients = []benchClient{
		{"python", []string{"./pyclient.py"}, false},
		{"golang", []string{"./goclient"}, false},
		{"nodejs", []string{"nodejs", "--expose-gc", "./jsclient.js"}, true},
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
	*benchClient

	// Number of times calibration has been performed.
	calibrations int

	// The cost of doing zero repetitions, indexed by concurrency.
	zeroCost []benchlib.Timing

	// The cost of a round w/ no working, no sleeping, no tracing.
	roundCost benchlib.Timing

	// The cost of a single unit of work.
	workCost benchlib.Timing

	// Cost of tracing a span that does no work
	spanCost benchlib.Timing

	spansReceived int64
	bytesReceived int64
}

type benchClient struct {
	Name string
	Args []string

	// When a client does not support preemtion and the CPU is
	// saturated, sleeping too long causes the test to run
	// overtime while sleeping too much.  To ensure the measured
	// timing is for the desired QPS, clients such as these (e.g.,
	// NodeJS) will have their sleep reduced.
	NeedsTimingAdjustment bool
}

func newBenchStats(bc *benchClient) *benchStats {
	return &benchStats{
		benchClient: bc,
		zeroCost:    make([]benchlib.Timing, maxConcurrency+1, maxConcurrency+1),
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
	glog.V(1).Info("Span creation cost: ", s.current.spanCost)
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
	glog.V(1).Info("Measuring work cost for ", multiplier, "..", 10*multiplier)
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
	glog.V(1).Infof("Work cost %.3gs/unit", s.current.workCost.Wall)
}

func (s *benchService) sanityCheckWork() bool {
	runfor := time.Second.Seconds()
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
		glog.Warning("CPU work not well calibrated (or insufficient CPU) @ concurrency ",
			conc, ": measured ", st.Mean(), " expected ", runfor,
			" off by ", absRatio*100.0, "%")
		return false
	}
	return true
}

func (s *benchService) measureTestLoop(trace bool) benchlib.Timing {
	// TODO combine this function with estimateWorkCost (same form)
	// Make sure the combined func is the only use of testTimeSlice.
	multiplier := int64(1000)
	for {
		tm := s.run(&benchlib.Control{
			Concurrent: 1,
			Work:       0,
			Repeat:     multiplier,
			Trace:      trace,
		})
		if tm.Adjusted.Wall.Seconds() > testTimeSlice.Seconds() {
			break
		}
		multiplier *= 10
	}
	// multiplier /= 10
	glog.V(1).Info("Measuring round cost for ", multiplier, "..", 10*multiplier)
	var data benchlib.Timings
	for i := 1; i <= 10; i++ {
		rounds := multiplier * int64(i)
		for j := 0; j < 10; j++ {
			// Note: This actives the amortized sleep
			// logic but never sleeps.
			tm := s.run(&benchlib.Control{
				Concurrent:    1,
				Work:          0,
				Sleep:         1,
				SleepInterval: time.Duration(rounds * 2),
				Repeat:        rounds,
				Trace:         trace,
			})
			data.Update(float64(rounds), tm.Adjusted)
		}
	}
	reg := data.LinearRegression()
	return reg.Slope()
}

func (s *benchService) measureSpanSaturation(opts saturationTest) benchlib.TimingStats {
	workTime := benchlib.Time(opts.load / opts.qps)
	sleepTime := benchlib.Time((1 - opts.load) / opts.qps)

	if benchlib.Time(workTime) < s.current.roundCost.Wall {
		// Too much test overhead to make an accurate measurement.
		glog.Fatal("Load is too low to hide test overhead")
		return benchlib.TimingStats{}
	}
	workTime -= s.current.roundCost.Wall
	total := opts.seconds * opts.qps

	tr := "untraced"
	if opts.trace {
		tr = "traced"
	}
	runOnce := func() (benchlib.TimingStats, stats.Stats, stats.Stats) {
		var ss benchlib.TimingStats
		var spans stats.Stats
		var bytes stats.Stats
		for ss.Count() != 5 {
			sbefore := s.current.spansReceived
			bbefore := s.current.bytesReceived
			tm := s.run(&benchlib.Control{
				Concurrent:  1,
				Work:        int64(workTime / s.current.workCost.Wall),
				Sleep:       time.Duration(sleepTime * nanosPerSecond),
				Repeat:      int64(total),
				Trace:       opts.trace,
				NumLogs:     opts.lognum,
				BytesPerLog: opts.logsize,
			})
			stotal := s.current.spansReceived - sbefore
			btotal := s.current.bytesReceived - bbefore

			glog.V(1).Infof("Trial %v@%3f%% %v (log%d*%d,%s,%.1f)",
				opts.qps, 100*opts.load, tm.Measured.Wall, opts.lognum, opts.logsize, tr,
				100.0*workTime/(workTime+sleepTime))

			glog.V(2).Info("Sleep total ", benchlib.Time(tm.Sleeps.Sum()),
				" i.e. ", tm.Sleeps.Sum()/opts.seconds*100.0, "%")

			ss.Update(tm.Measured)
			spans.Update(float64(stotal))
			bytes.Update(float64(btotal))
		}
		return ss, spans, bytes
	}
	for {
		if s.current.NeedsTimingAdjustment ||
			s.current.calibrations < minimumCalibrations {
			// Adjust for on-the-fly compilation,
			// initialization costs, etc.
			s.recalibrate()
		}

		ss, spans, bytes := runOnce()
		if s.current.NeedsTimingAdjustment && sleepTime != 0 {
			offBy := ss.Wall.Mean() - opts.seconds
			ratio := offBy / opts.seconds
			if math.Abs(ratio) > timingTolerance {
				adjust := benchlib.Time(offBy / float64(total))
				if sleepTime < adjust {
					sleepTime = 0
				} else {
					sleepTime -= adjust
				}
				glog.V(1).Info("Adjust timing by ", -adjust, " (", sleepTime+adjust, " to ",
					sleepTime, ") diff ", offBy)
				continue
			}
		}
		glog.Infof("Load %v@%3f%% %v (log%d*%d,%s,%.1f%%) == %.2f%% %.2fB/span",
			opts.qps, 100*opts.load, ss, opts.lognum, opts.logsize, tr,
			100.0*workTime/(workTime+sleepTime),
			(spans.Mean()/total)*100, bytes.Mean()/spans.Mean())
		return ss
	}
}

func (s *benchService) measureImpairment() {
	// Each test runs this long.
	const testTime = 30

	// Test will compute CPU tax measure for each QPS listed
	qpss := []float64{
		100,
		200,
		300, 400, 500,
		600, 700, 800, 900, 1000,
	}
	logcfg := []struct{ num, size int64 }{
		{0, 0},
		{2, 100},
		{4, 100},
		{6, 100},
	}
	loadlist := []float64{
		.5, .6, .7, .8, .9,
		.92, .94, .96, .98,
		.99, .995, .997, .999, 1.0,
	}
	for _, qps := range qpss {
		for _, lcfg := range logcfg {
			for _, load := range loadlist {
				s.measureSpanSaturation(saturationTest{
					trace:   false,
					seconds: testTime,
					qps:     qps,
					load:    load,
					lognum:  lcfg.num,
					logsize: lcfg.size})
				s.measureSpanSaturation(saturationTest{
					trace:   true,
					seconds: testTime,
					qps:     qps,
					load:    load,
					lognum:  lcfg.num,
					logsize: lcfg.size})
			}
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

func (s *benchService) run(c *benchlib.Control) *benchlib.Result {
	if c.SleepInterval == 0 {
		c.SleepInterval = benchlib.DefaultSleepInterval
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

func (s *benchService) recalibrate() {
	cnt := s.current.calibrations
	s.current = newBenchStats(s.current.benchClient)
	s.current.calibrations = cnt + 1
	s.warmup()
	s.estimateZeroCosts()
	s.estimateRoundCost()
	s.estimateWorkCost()
	for !s.sanityCheckWork() {
		s.estimateWorkCost()
	}
	s.measureSpanCost()
}

func (s *benchService) runTest(bc *benchClient) {
	s.current = newBenchStats(bc)

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

	s.recalibrate()
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
		glog.V(2).Info("Sleep timing: mean ", benchlib.Time(sstat.Mean()),
			" stddev ", benchlib.Time(sstat.PopulationStandardDeviation()))
		glog.V(3).Info("Sleep values: ", sleep_info)
	}
	s.resultCh <- &benchlib.Result{
		Measured: benchlib.Timing{
			Wall: benchlib.ParseTime(params.Get("timing")),
			User: usage.User,
			Sys:  usage.Sys,
		},
		Flush: benchlib.Timing{
			Wall: benchlib.ParseTime(params.Get("flush")),
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
