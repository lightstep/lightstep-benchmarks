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

// TODO remove the if (Sleep != 0) test from each loadtest client, remove
// the hacky Sleep = 1; SleepInterval = BIG; hack in this file.

const (
	// collectorBinaryPath is the path of the Thrift collector service
	collectorBinaryPath = "/_rpc/v1/reports/binary"
	// collectorJSONPath is the path of the pure-JSON collector service
	collectorJSONPath = "/api/v0/reports"

	// testIteration is used for initial estimates and calibration.
	testIteration = 1000

	// maxConcurrency is the limit of concurrency testing
	maxConcurrency = 1

	// testTolerance is used for a sanity checks.
	testTolerance = 0.02

	// For clients that require timing correction (i.e., NodeJS)
	timingTolerance = 0.005 // ~= .3 second per minute

	nanosPerSecond = 1e9

	minimumCalibrations = 3

	experimentRounds = 3

	// testTimeSlice is a small duration used to set a minimum
	// reasonable execution time during calibration.
	testTimeSlice = time.Second / 2

	testRounds = 7
)

var (
	// client is a list of client programs for the benchmark
	allClients = []benchClient{
		{"cpp", []string{"./github.com/lightstep/lightstep-tracer-cpp/test/cppclient"}},
		{"ruby", []string{"ruby", "./rbclient.rb"}},
		{"python", []string{"./pyclient.py"}},
		{"golang", []string{"./goclient"}},
		{"nodejs", []string{"node",
			"--expose-gc",
			"--always_opt",
			//"--trace-gc", "--trace-gc-verbose", "--trace-gc-ignore-scavenger",
			"./jsclient.js"}},
	}

	// requestCh is used to serialize HTTP requests
	requestCh = make(chan sreq)

	headless = flag.Bool("headless", true,
		"If true, this process will the run the test clients "+
			" itself. Otherwise, tests may be run manually by setting "+
			"this false.")
	client = flag.String("client", "golang",
		"Name of the client library being tested (matches benchClient.Name)")
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
			st.Update(tm.Measured)
		}
		s.current.zeroCost[c] = st.Mean()
		glog.V(1).Info("Cost Z_c_", c, " = ", s.current.zeroCost[c])
	}
}

// measureSpanCost runs a closed loop creating a certain
// number of spans as quickly as possible and reporting
// the timing.
func (s *benchService) measureSpanCost() {
	s.current.spanCost = s.measureTestLoop(true)
	glog.V(1).Infof("Cost T = %s/span", s.current.spanCost)
}

// estimateRoundCost runs a untraced loop doing no work to establish
// the baseline cost of a repetition.
func (s *benchService) estimateRoundCost() {
	s.current.roundCost = s.measureTestLoop(false)
	glog.V(1).Infof("Cost R = %s/round", s.current.roundCost)
}

// estimateWorkCosts measures the cost of the work function.
// TODO this body is now nearly identical to measureTestLoop; Fix.
func (s *benchService) estimateWorkCost() {
	// The work function is assumed to be fast. Find a multiplier
	// that results in working at least testTimeSlice.
	multiplier := int64(1000)
	for {
		glog.V(2).Info("Testing work for rounds=", multiplier)
		tm := s.run(&benchlib.Control{
			Concurrent:    1,
			Work:          multiplier,
			Repeat:        1,
			Sleep:         1,
			SleepInterval: time.Duration(2),
		})
		if tm.Measured.Wall.Seconds() < testTimeSlice.Seconds() {
			multiplier *= 10
			continue
		}
		var st benchlib.TimingStats
		for j := 0; j < testRounds; j++ {
			glog.V(2).Info("Measuring work for rounds=", multiplier)
			tm := s.run(&benchlib.Control{
				Concurrent:    1,
				Work:          multiplier,
				Repeat:        1,
				Sleep:         1,
				SleepInterval: time.Duration(2),
			})
			adjusted := tm.Measured.Sub(s.current.zeroCost[1]).Sub(s.current.roundCost)
			st.Update(adjusted)
			glog.V(1).Info("Measured work for rounds=", multiplier, " in ", adjusted,
				" == ", float64(adjusted.Wall)/float64(multiplier))
		}
		s.current.workCost = st.Mean().Div(float64(multiplier))
		glog.V(1).Infof("Cost W = %s/unit", s.current.workCost)
		return
	}
}

func (s *benchService) sanityCheckWork() bool {
	var st benchlib.TimingStats
	for i := 0; i < testRounds; i++ {
		work := int64(testTimeSlice.Seconds() / s.current.workCost.Wall.Seconds())
		tm := s.run(&benchlib.Control{
			Concurrent:    1,
			Work:          work,
			Repeat:        1,
			Sleep:         1,
			SleepInterval: time.Duration(2),
		})
		adjusted := tm.Measured.Sub(s.current.zeroCost[1]).Sub(s.current.roundCost)
		st.Update(adjusted)
	}
	glog.V(1).Infoln("Check work timing", st, "expected", testTimeSlice)

	absRatio := math.Abs((st.Wall.Mean() - testTimeSlice.Seconds()) / testTimeSlice.Seconds())
	if absRatio > testTolerance {
		glog.Warning("CPU work not well calibrated (or insufficient CPU): measured ",
			st.Mean(), " expected ", testTimeSlice,
			" off by ", absRatio*100.0, "%")
		return false
	}
	return true
}

func (s *benchService) measureTestLoop(trace bool) benchlib.Timing {
	multiplier := int64(1000)
	for {
		glog.V(2).Info("Measuring loop for rounds=", multiplier)
		tm := s.run(&benchlib.Control{
			Concurrent:    1,
			Work:          0,
			Sleep:         1,
			SleepInterval: time.Duration(multiplier * 2),
			Repeat:        multiplier,
			Trace:         trace,
		})
		if tm.Measured.Wall.Seconds() < testTimeSlice.Seconds() {
			multiplier *= 10
			continue
		}
		var ss benchlib.TimingStats
		for j := 0; j < testRounds; j++ {
			tm := s.run(&benchlib.Control{
				Concurrent:    1,
				Work:          0,
				Sleep:         1,
				SleepInterval: time.Duration(multiplier * 2),
				Repeat:        multiplier,
				Trace:         trace,
			})
			adjusted := tm.Measured.Sub(s.current.zeroCost[1])
			if trace {
				adjusted = adjusted.SubFactor(s.current.roundCost, float64(multiplier))
			}
			ss.Update(adjusted)
			glog.V(2).Info("Measured cost for rounds=", multiplier, " in ", adjusted,
				" == ", float64(adjusted.Wall)/float64(multiplier))
		}
		return ss.Mean().Div(float64(multiplier))
	}
}

// Returns the CPU impairment as a ratio (e.g., 0.01 for 1% impairment).
func (s *benchService) measureSpanSaturation(opts saturationTest) *float64 {
	workTime := benchlib.Time(opts.load / opts.qps)
	sleepTime := benchlib.Time((1 - opts.load) / opts.qps)
	sleepTime0 := sleepTime
	total := opts.seconds * opts.qps

	tr := "untraced"
	if opts.trace {
		tr = "traced"
	}
	runOnce := func() (*benchlib.TimingStats, *stats.Stats, *stats.Stats, *stats.Stats) {
		var ss benchlib.TimingStats
		var spans stats.Stats
		var bytes stats.Stats
		var sleeps stats.Stats
		for ss.Count() != experimentRounds {
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
			actualSleep := tm.Sleeps.Sum()

			adjusted := tm.Measured.Sub(s.current.zeroCost[1]).SubFactor(s.current.roundCost, total)
			impairment := (adjusted.Wall.Seconds() - (total * workTime.Seconds()) - actualSleep) / adjusted.Wall.Seconds()

			glog.V(1).Infof("Trial %v@%3f%% %v (log%d*%d,%s) impairment %.2f%% (sleep time %s)",
				opts.qps, 100*opts.load, tm.Measured.Wall, opts.lognum, opts.logsize, tr,
				100*impairment, sleepTime)

			// If more than 10% under, recalibrate
			if impairment < -0.1 {
				return nil, nil, nil, nil
			}

			glog.V(2).Info("Sleep total ", benchlib.Time(actualSleep),
				" i.e. ", actualSleep/opts.seconds*100.0, "%")

			ss.Update(adjusted)
			sleeps.Update(actualSleep)
			spans.Update(float64(stotal))
			bytes.Update(float64(btotal))
		}
		return &ss, &spans, &bytes, &sleeps
	}
	for {
		if s.current.calibrations < minimumCalibrations {
			// Adjust for on-the-fly compilation,
			// initialization costs, etc.
			s.recalibrate()
		}

		ss, spans, bytes, sleeps := runOnce()

		if ss == nil {
			s.recalibrate()
			continue
		}

		// TODO The logic here is using averages, which allows
		// several wildly-out-of-range runs to counter each
		// other.  Perform this fitness test on individual run
		// times.
		offBy := ss.Wall.Mean() - opts.seconds
		ratio := offBy / opts.seconds
		if math.Abs(ratio) > timingTolerance {
			adjust := -benchlib.Time(offBy / float64(total))
			if adjust < 0 {
				if sleepTime == 0 {
					// The load factor precludes this test from succeeding.
					glog.Info("Load factor is too high to continue")
					return nil
				}
				if sleepTime+adjust <= 0 {
					glog.V(1).Info("Adjust timing to zero (", sleepTime, " adjust ", adjust, ") off by ", offBy)
					sleepTime = 0
					continue
				}
			}

			glog.V(1).Info("Adjust timing by ", adjust, " (", sleepTime, " to ",
				sleepTime+adjust, ") off by ", offBy)
			sleepTime += adjust

			if sleepTime > sleepTime0 {
				sleepTime = sleepTime0
				s.recalibrate()
			}
			continue
		}

		impairment := (ss.Wall.Mean() - (total * workTime.Seconds()) - sleeps.Mean()) / ss.Wall.Mean()
		glog.Infof("Load %v@%3f%% %v (log%d*%d,%s) impaired %.2f%% completed %.2f%% @ %.2fB/span",
			opts.qps, 100*opts.load, ss, opts.lognum, opts.logsize, tr,
			100*impairment,
			100*(spans.Mean()/total),
			bytes.Mean()/spans.Mean())
		return &impairment
	}
}

func (s *benchService) measureImpairment() {
	// Each test runs this long.
	const testTime = 600

	// Test will compute CPU tax measure for each QPS listed
	qpss := []float64{
		100,
		500,
		1000,
	}
	loadlist := []float64{
		.8,
	}
	// THEN add an experiment to ramp up collector latency and observe
	// increase in impairment? dropped spans? tolerance? etc.
	for _, qps := range qpss {
		for _, load := range loadlist {
			glog.Infof("Starting %v@%3f%%", qps, 100*load)
			untracedImpaired := s.measureSpanSaturation(saturationTest{
				trace:   false,
				seconds: testTime,
				qps:     qps,
				load:    load,
				//lognum:  lcfg.num,
				//logsize: lcfg.size,
			})
			if untracedImpaired == nil {
				continue
			}
			tracedImpaired := s.measureSpanSaturation(saturationTest{
				trace:   true,
				seconds: testTime,
				qps:     qps,
				load:    load,
				//lognum:  lcfg.num,
				//logsize: lcfg.size,
			})
			if tracedImpaired == nil {
				continue
			}
			glog.Infof("Load %v@%3f%%: Tracing adds %.02f%% CPU impairment",
				qps, 100*load, 100*(*tracedImpaired-*untracedImpaired))
		}
	}
}

func (s *benchService) warmup() {
	s.run(&benchlib.Control{
		Concurrent:    1,
		Work:          1000,
		Repeat:        10,
		Trace:         false,
		Sleep:         1,
		SleepInterval: 5,
	})
	s.run(&benchlib.Control{
		Concurrent:    1,
		Work:          1000,
		Repeat:        10,
		Trace:         true,
		Sleep:         10,
		SleepInterval: 100,
	})
}

func (s *benchService) run(c *benchlib.Control) *benchlib.Result {
	if c.SleepInterval == 0 {
		c.SleepInterval = benchlib.DefaultSleepInterval
	}
	glog.V(3).Info("Next control: ", c)
	s.controlCh <- c
	// TODO: Maybe timeout here and help diagnose hung process?
	r := <-s.resultCh
	glog.V(3).Info("Measured: ", r.Measured, " using ", s.current)
	return r
}

func (s *benchService) runTests() {
	if !*headless {
		s.runTest(nil)
	} else {
		ran := false
		for _, bc := range allClients {
			if bc.Name == *client {
				s.runTest(&bc)
				ran = true
			}
		}
		if !ran {
			glog.Fatal("Please set --client=<...>")
		}
	}
	os.Exit(0)
}

func (s *benchService) recalibrate() {
	for {
		glog.V(1).Info("Calibration starting")
		cnt := s.current.calibrations
		s.current = newBenchStats(s.current.benchClient)
		s.current.calibrations = cnt + 1
		s.warmup()
		s.estimateZeroCosts()
		s.estimateRoundCost()
		s.estimateWorkCost()
		if !s.sanityCheckWork() {
			continue
		}
		s.measureSpanCost()
		return
	}
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
				glog.Fatal("Could not parse timing: ", sleep_info)
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
	// The response body is not used, but some HTTP clients are
	// troubled by 0-byte responses.
	res.Write([]byte("OK"))
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
		ReadTimeout:  86400 * time.Second,
		WriteTimeout: 86400 * time.Second,
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
