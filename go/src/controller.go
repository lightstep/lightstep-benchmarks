package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/cloud"
	"google.golang.org/cloud/storage"
	"google.golang.org/grpc"

	bench "benchlib"

	cpb "github.com/lightstep/lightstep-tracer-go/collectorpb"
	lst "github.com/lightstep/lightstep-tracer-go/lightstep_thrift"
	"github.com/lightstep/lightstep-tracer-go/thrift_0_9_2/lib/go/thrift"
)

// TODO remove the if (Sleep != 0) test from each loadtest client
// (should use a <=, see goclient, jsclient, pyclient. Remove the
// hacky Sleep = 1; SleepInterval = BIG; hack in this file.

// TODO parameterize the test constants below so it's possible to
// run short and long tests easily.
// type TestQuality struct {
// }

const (
	// collectorBinaryPath is the path of the Thrift collector service
	collectorBinaryPath = "/_rpc/v1/reports/binary"
	// collectorJSONPath is the path of the pure-JSON collector service
	collectorJSONPath = "/api/v0/reports"

	nanosPerSecond = 1e9

	// testIteration is used for initial estimates and calibration.
	testIteration = 1000

	// maxConcurrency is the limit of concurrency testing
	maxConcurrency = 1

	// testTolerance is used for a sanity checks.
	testTolerance = 0.01

	// TODO This is hacky, since sleep calibration doesn't use this go fast.
	minimumCalibrations = 1
	calibrateRounds     = 20

	// testTimeSlice is a small duration used to set a minimum
	// reasonable execution time during calibration.
	testTimeSlice = 50 * time.Millisecond

	// If the test runs more than 1% faster than theoretically
	// possible, recalibrate.
	negativeRecalibrationThreshold = -0.01

	// Parameters for measuring impairment
	experimentDuration = 120
	experimentRounds   = 40

	minimumRate    = 100
	maximumRate    = 1000
	rateIncrements = 9

	minimumLoad    = 0.5
	maximumLoad    = 1.0
	loadIncrements = 10

	// Sleep experiment parameters
	sleepTrialCount     = 1000
	sleepRepeats        = int64(10)
	sleepMinWorkFactor  = int64(10)
	sleepMaxWorkFactor  = int64(100)
	sleepWorkFactorIncr = int64(90)
)

var (
	allClients = map[string]benchClient{
		"cpp":    {[]string{"./cppclient"}},
		"ruby":   {[]string{"ruby", "./rbclient.rb"}},
		"python": {[]string{"./pyclient.py"}},
		"golang": {[]string{"./goclient"}},
		"nodejs": {[]string{"node",
			"--expose-gc",
			"--always_opt",
			//"--trace-gc", "--trace-gc-verbose", "--trace-gc-ignore-scavenger",
			"./jsclient.js"}},
		"java": {[]string{
			"java",
			// "-classpath",
			// "lightstep-benchmark-0.1.28.jar",
			// "-Xdebug", "-Xrunjdwp:transport=dt_socket,address=7000,server=y,suspend=n",
			"com.lightstep.benchmark.BenchmarkClient"}},
	}

	// requestCh is used to serialize HTTP requests
	requestCh = make(chan sreq)

	testStorageBucket = getEnv("BENCHMARK_BUCKET", "lightstep-client-benchmarks")
	testTitle         = getEnv("BENCHMARK_TITLE", "untitled")
	testConfigName    = getEnv("BENCHMARK_CONFIG_NAME", "unnamed")
	testConfigFile    = getEnv("BENCHMARK_CONFIG_FILE", "config.json")
	testClient        = getEnv("BENCHMARK_CLIENT", "unknown")
	testZone          = getEnv("BENCHMARK_ZONE", "")
	testProject       = getEnv("BENCHMARK_PROJECT", "")
	testInstance      = getEnv("BENCHMARK_INSTANCE", "")
	testVerbose       = getEnv("BENCHMARK_VERBOSE", "")
)

type impairmentTest struct {
	// Configuration
	concurrency int
	lognum      int64
	logsize     int64

	// Experiment variables
	trace bool
	rate  float64
	load  float64
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
	controlCh        chan *bench.Control
	resultCh         chan *bench.Result
	storage          *storage.Client
	bucket           *storage.BucketHandle
	gcpClient        *http.Client

	// outstanding request state
	controlling bool
	before      bench.Timing

	// current collects results for the current test
	current *benchStats
}

type benchStats struct {
	benchClient

	// Number of times calibration has been performed.
	calibrations int

	// The cost of doing zero repetitions, indexed by concurrency.
	// Note: this is a small, sparse array because we only test
	// power-of-two configurations.
	zeroCost []bench.Timing

	// The cost of a round w/ no working, no sleeping, no tracing.
	roundCost bench.Timing

	// The cost of a single unit of work.
	workCost bench.Timing

	// Cost of tracing a span that does no work.
	spanCost bench.Timing

	spansReceived int64
	spansDropped  int64
	bytesReceived int64
}

type benchClient struct {
	Args []string
}

func fatal(x ...interface{}) {
	panic(fmt.Sprintln(x...))
}

func print(x ...interface{}) {
	if testVerbose == "true" {
		fmt.Sprintln(x...)
	}
}

func getEnv(name, defval string) string {
	if r := os.Getenv(name); r != "" {
		return r
	}
	return defval
}

func newBenchStats(bc benchClient) *benchStats {
	return &benchStats{
		benchClient: bc,
		zeroCost:    make([]bench.Timing, maxConcurrency+1, maxConcurrency+1),
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
	s.countDroppedSpans(request)
	return fakeReportResponse(), nil
}

func (s *benchService) countDroppedSpans(request *lst.ReportRequest) {
	if request.InternalMetrics == nil {
		return
	}
	for _, c := range request.InternalMetrics.Counts {
		if c.Name == "spans.dropped" {
			if c.Int64Value != nil {
				s.current.spansDropped += *c.Int64Value
			} else if c.DoubleValue != nil {
				s.current.spansDropped += int64(*c.DoubleValue)
			}
		}
	}
}

// Note: This is a duplicate of countDroppedSpans for a protobuf
// Report instead of a Thrift report.
func (s *benchService) countGrpcDroppedSpans(request *cpb.ReportRequest) {
	if request.InternalMetrics == nil {
		return
	}
	for _, c := range request.InternalMetrics.Counts {
		if c.Name == "spans.dropped" {
			switch t := c.Value.(type) {
			case *cpb.MetricsSample_IntValue:
				s.current.spansDropped += t.IntValue
			case *cpb.MetricsSample_DoubleValue:
				s.current.spansDropped += int64(t.DoubleValue)
			}
		}
	}
}

// BytesReceived is called from the HTTP layer before Thrift
// processing, recording inbound byte count.
func (s *benchService) BytesReceived(num int64) {
	s.current.bytesReceived += num
}

// estimateZeroCosts measures the cost of doing nothing.
func (s *benchService) estimateZeroCosts() {
	for c := 1; c <= maxConcurrency; c *= 2 {
		var st bench.TimingStats
		for j := 0; j < testIteration; j++ {
			tm := s.run(&bench.Control{
				Concurrent: c,
			})
			st.Update(tm.Measured)
		}
		s.current.zeroCost[c] = st.Mean()
		print("Cost Z_c_", c, " = ", s.current.zeroCost[c])
	}
}

// measureSpanCost runs a closed loop creating a certain
// number of spans as quickly as possible and reporting
// the timing.
func (s *benchService) measureSpanCost() {
	s.current.spanCost = s.measureTestLoop(true)
	print("Cost T =", s.current.spanCost, "/span")
}

// estimateRoundCost runs a untraced loop doing no work to establish
// the baseline cost of a repetition.
func (s *benchService) estimateRoundCost() {
	s.current.roundCost = s.measureTestLoop(false)
	print("Cost R =", s.current.roundCost, "/round")
}

// estimateWorkCosts measures the cost of the work function.
// TODO this body is now nearly identical to measureTestLoop; Fix.
func (s *benchService) estimateWorkCost() {
	// The work function is assumed to be fast. Find a multiplier
	// that results in working at least testTimeSlice.
	multiplier := int64(1000)
	for {
		print("Testing work for rounds=", multiplier)
		tm := s.run(&bench.Control{
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
		var st bench.TimingStats
		for j := 0; j < calibrateRounds; j++ {
			print("Measuring work for rounds=", multiplier)
			tm := s.run(&bench.Control{
				Concurrent:    1,
				Work:          multiplier,
				Repeat:        1,
				Sleep:         1,
				SleepInterval: time.Duration(2),
			})
			adjusted := tm.Measured.Sub(s.current.zeroCost[1]).Sub(s.current.roundCost)
			st.Update(adjusted)
			print("Measured work for rounds=", multiplier, " in ", adjusted,
				" == ", float64(adjusted.Wall)/float64(multiplier))
		}
		s.current.workCost = st.Mean().Div(float64(multiplier))
		print("Cost W =", s.current.workCost, "/unit")
		return
	}
}

func (s *benchService) sanityCheckWork() bool {
	var st bench.TimingStats
	for i := 0; i < calibrateRounds; i++ {
		work := int64(testTimeSlice.Seconds() / s.current.workCost.Wall.Seconds())
		tm := s.run(&bench.Control{
			Concurrent:    1,
			Work:          work,
			Repeat:        1,
			Sleep:         1,
			SleepInterval: time.Duration(2),
		})
		adjusted := tm.Measured.Sub(s.current.zeroCost[1]).Sub(s.current.roundCost)
		st.Update(adjusted)
	}
	print("Check work timing", st, "expected", testTimeSlice)

	absRatio := math.Abs((st.Wall.Mean() - testTimeSlice.Seconds()) / testTimeSlice.Seconds())
	if absRatio > testTolerance {
		fmt.Println("WARNING: CPU work not well calibrated (or insufficient CPU): measured ",
			st.Mean(), " expected ", testTimeSlice,
			" off by ", absRatio*100.0, "%")
		return false
	}
	return true
}

func (s *benchService) measureTestLoop(trace bool) bench.Timing {
	multiplier := int64(1000)
	for {
		print("Measuring loop for rounds=", multiplier)
		tm := s.run(&bench.Control{
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
		var ss bench.TimingStats
		for j := 0; j < calibrateRounds; j++ {
			tm := s.run(&bench.Control{
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
			print("Measured cost for rounds=", multiplier, " in ", adjusted,
				" == ", float64(adjusted.Wall)/float64(multiplier))
		}
		return ss.Mean().Div(float64(multiplier))
	}
}

func (s *benchService) measureSpanImpairment(opts impairmentTest) (bench.DataPoint, float64) {
	qpsPerCpu := opts.rate / float64(opts.concurrency)

	workTime := bench.Time(opts.load / qpsPerCpu)
	sleepTime := bench.Time((1 - opts.load) / qpsPerCpu)
	totalSpans := opts.rate * experimentDuration
	totalPerCpu := experimentDuration * qpsPerCpu

	tr := "untraced"
	if opts.trace {
		tr = "traced"
	}
	runOnce := func() (runtime *bench.Timing, spans, dropped, bytes int64, rate, work, sleep float64) {
		sbefore := s.current.spansReceived
		bbefore := s.current.bytesReceived
		dbefore := s.current.spansDropped
		tm := s.run(&bench.Control{
			Concurrent:  opts.concurrency,
			Work:        int64(workTime / s.current.workCost.Wall),
			Sleep:       time.Duration(sleepTime * nanosPerSecond),
			Repeat:      int64(totalPerCpu),
			Trace:       opts.trace,
			NumLogs:     opts.lognum,
			BytesPerLog: opts.logsize,
		})
		stotal := s.current.spansReceived - sbefore
		btotal := s.current.bytesReceived - bbefore
		dtotal := s.current.spansDropped - dbefore

		adjusted := tm.Measured.Sub(s.current.zeroCost[opts.concurrency]).SubFactor(s.current.roundCost, totalPerCpu)
		sleepPerCpu := tm.Sleeps.Seconds() / float64(opts.concurrency)
		workPerCpu := totalPerCpu * workTime.Seconds()
		actualRate := totalSpans / adjusted.Wall.Seconds()
		traceCost := adjusted.Wall.Seconds() - workPerCpu - sleepPerCpu
		impairment := traceCost / adjusted.Wall.Seconds()
		workLoad := workPerCpu / adjusted.Wall.Seconds()
		sleepLoad := sleepPerCpu / adjusted.Wall.Seconds()
		visibleLoad := (adjusted.Wall.Seconds() - sleepPerCpu) / adjusted.Wall.Seconds()

		print(fmt.Sprintf("Trial %v@%3f%% %v (log%d*%d,%s) work load %.2f%% visible load %.2f%% visible impairment %.2f%%, actual rate %.1f",
			opts.rate, 100*opts.load, adjusted.Wall, opts.lognum, opts.logsize, tr,
			100*workLoad, 100*visibleLoad, 100*impairment, actualRate))

		// If too far under, recalibrate
		if impairment < negativeRecalibrationThreshold {
			return nil, 0, 0, 0, 0, 0, 0
		}

		return &adjusted, stotal, dtotal, btotal, actualRate, workLoad, sleepLoad
	}
	for {
		if s.current.calibrations < minimumCalibrations {
			// Adjust for on-the-fly compilation,
			// initialization costs, etc.
			s.recalibrate()
		}

		ss, spans, _, _, actualRate, workLoad, sleepLoad := runOnce()

		if ss == nil {
			s.recalibrate()
			continue
		}

		completeRatio := float64(spans) / totalSpans
		return bench.DataPoint{actualRate, workLoad, sleepLoad}, completeRatio
	}
}

func (s *benchService) measureImpairment(c bench.Config) {
	output := bench.Output{}
	output.Title = testTitle
	output.Client = testClient
	output.Name = testConfigName
	output.Concurrent = c.Concurrency
	output.LogBytes = c.LogNum * c.LogSize

	rateInterval := float64(maximumRate-minimumRate) / rateIncrements

	for rate := float64(minimumRate); rate <= maximumRate; rate += rateInterval {
		loadInterval := float64(maximumLoad-minimumLoad) / loadIncrements
		for load := minimumLoad; load <= maximumLoad; load += loadInterval {
			m := s.measureImpairmentAtRateAndLoad(c, rate, load)
			output.Results = append(output.Results, m...)
		}
	}
}

func (s *benchService) saveResult(result bench.Output) {
	encoded, err := json.MarshalIndent(result, "", "")
	if err != nil {
		fatal("Couldn't encode JSON!", err)
	}
	withNewline := append(encoded, '\n')
	fmt.Print(string(withNewline))
	s.writeTo(path.Join(result.Title, result.Name, result.Client), withNewline)
}

func (s *benchService) writeTo(name string, data []byte) {
	object := s.bucket.Object(name)
	w := object.NewWriter(context.Background())
	_, err := w.Write(data)
	if err != nil {
		fatal("Couldn't write storage bucket!", err)
	}
	err = w.Close()
	if err != nil {
		fatal("Couldn't close storage bucket! ", err)
	}
}

func (s *benchService) measureImpairmentAtRateAndLoad(c bench.Config, rate, load float64) []bench.Measurement {
	print(fmt.Sprintf("Starting rate=%.2f/sec load=%.2f%% test", rate, load*100))
	ms := []bench.Measurement{}
	for i := 0; i < experimentRounds; i++ {
		m := bench.Measurement{}
		m.TargetRate = rate
		m.TargetLoad = load
		m.Untraced, _ = s.measureSpanImpairment(impairmentTest{
			trace:       false,
			concurrency: c.Concurrency,
			rate:        rate,
			load:        load,
			lognum:      c.LogNum,
			logsize:     c.LogSize,
		})
		m.Traced, m.Completion = s.measureSpanImpairment(impairmentTest{
			trace:       true,
			concurrency: c.Concurrency,
			rate:        rate,
			load:        load,
			lognum:      c.LogNum,
			logsize:     c.LogSize,
		})
		ms = append(ms, m)
	}
	return ms
}

func (s *benchService) estimateSleepCosts(_ bench.Config, o *bench.Output) {
	print("Estimating sleep cost")

	equalWork := int64(bench.DefaultSleepInterval.Seconds() / s.current.workCost.Wall.Seconds())

	type sleepTrial struct {
		with    bench.TimingStats
		without bench.TimingStats
		sleeps  bench.TimingStats
	}

	var sleepTrials []sleepTrial

	for m := sleepMinWorkFactor; m <= sleepMaxWorkFactor; m += sleepWorkFactorIncr {
		var st sleepTrial
		for i := 0; i < sleepTrialCount; i++ { // TODO should be ... until 95% confidence or at least N
			wsleep := s.run(&bench.Control{
				Concurrent: 1, // TODO for now..., need to test >1
				Work:       equalWork * m,
				Sleep:      bench.DefaultSleepInterval,
				Repeat:     sleepRepeats,
			})
			ssleep := s.run(&bench.Control{
				Concurrent: 1,
				Work:       equalWork * m,
				Sleep:      0,
				Repeat:     sleepRepeats,
			})

			st.with.Update(wsleep.Measured)
			st.without.Update(ssleep.Measured)
			st.sleeps.Update(bench.Timing{Wall: wsleep.Sleeps})

			o.Sleeps = append(o.Sleeps, bench.SleepCalibration{
				WorkFactor:  int(m),
				RunAndSleep: wsleep.Measured.Wall.Seconds(),
				RunNoSleep:  ssleep.Measured.Wall.Seconds(),
				ActualSleep: wsleep.Sleeps.Seconds(),
				Repeats:     int(sleepRepeats),
			})
		}
		fmt.Println("Work factor", m, "sleep cost",
			st.with.Mean().
				Sub(st.without.Mean()).
				Sub(st.sleeps.Mean()).
				Div(float64(sleepRepeats)))

		sleepTrials = append(sleepTrials, st)
	}
}

func (s *benchService) warmup() {
	s.run(&bench.Control{
		Concurrent:    1,
		Work:          1000,
		Repeat:        10,
		Trace:         false,
		Sleep:         1,
		SleepInterval: 5,
	})
	s.run(&bench.Control{
		Concurrent:    1,
		Work:          1000,
		Repeat:        10,
		Trace:         true,
		Sleep:         10,
		SleepInterval: 100,
	})
}

func (s *benchService) run(c *bench.Control) *bench.Result {
	if c.SleepInterval == 0 {
		c.SleepInterval = bench.DefaultSleepInterval
	}
	s.controlCh <- c
	// TODO: Maybe timeout here and help diagnose hung process?
	r := <-s.resultCh
	return r
}

func (s *benchService) runTests(b benchClient, c bench.Config) {
	s.runTest(b, c)
	s.tearDown()
}

func (s *benchService) recalibrate() {
	for {
		print("Calibration starting")
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

func (s *benchService) runTest(bc benchClient, c bench.Config) {
	s.current = newBenchStats(bc)

	print("Testing ", testClient)
	ch := make(chan bool)

	defer func() {
		s.exitClient()
		<-ch
	}()

	go s.execClient(bc, ch)

	s.recalibrate()

	output := bench.Output{}
	output.Title = testTitle
	output.Client = testClient
	output.Name = testConfigName
	output.Concurrent = c.Concurrency
	output.LogBytes = c.LogNum * c.LogSize

	s.estimateSleepCosts(c, &output)
	//s.measureImpairment(c, &output)

	s.saveResult(output)
}

func (s *benchService) execClient(bc benchClient, ch chan bool) {
	cmd := exec.Command(bc.Args[0], bc.Args[1:]...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Start(); err != nil {
		fatal("Could not start client: ", err)
	}
	if err := cmd.Wait(); err != nil {
		perr, ok := err.(*exec.ExitError)
		if !ok {
			fatal("Could not await client: ", err)
		}
		if !perr.Exited() {
			fatal("Client did not exit: ", err)
		}
		if !perr.Success() {
			fatal("Client failed: ", string(perr.Stderr))
		}
	}
	ch <- true
}

func (s *benchService) exitClient() {
	s.controlCh <- &bench.Control{Exit: true}
	s.controlling = false
}

// ServeControlHTTP returns a JSON control request to the client.
func (s *benchService) ServeControlHTTP(res http.ResponseWriter, req *http.Request) {
	if s.controlling {
		fatal("Out-of-phase control request", req.URL)
	}
	s.before = bench.GetChildUsage()
	s.controlling = true
	body, err := json.Marshal(<-s.controlCh)
	if err != nil {
		fatal("Marshal error: ", err)
	}
	res.Write(body)
}

// ServeResultHTTP records the client's result via a URL Query parameter "timing".
func (s *benchService) ServeResultHTTP(res http.ResponseWriter, req *http.Request) {
	if !s.controlling {
		fatal("Out-of-phase client result", req.URL)
	}
	usage := bench.GetChildUsage().Sub(s.before)
	// Note: it would be nice if there were a decoder to unmarshal
	// from URL query param into bench.Result, e.g., opposite of
	// https://godoc.org/github.com/google/go-querystring/query
	params, err := url.ParseQuery(req.URL.RawQuery)

	if err != nil {
		fatal("Error parsing URL params: ", req.URL.RawQuery)
	}
	s.controlling = false
	s.resultCh <- &bench.Result{
		Measured: bench.Timing{
			Wall: bench.ParseTime(params.Get("timing")),
			User: usage.User,
			Sys:  usage.Sys,
		},
		Flush: bench.Timing{
			Wall: bench.ParseTime(params.Get("flush")),
		},
		Sleeps: bench.ParseTime(params.Get("s")),
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

	s.countDroppedSpans(reportRequest)

	res.Header().Set("Content-Type", "application/json")
	if err = json.NewEncoder(res).Encode(fakeReportResponse()); err != nil {
		http.Error(res, "Unable to encode response: "+err.Error(), http.StatusBadRequest)
	}
}

func (s *benchService) ServeDefaultHTTP(res http.ResponseWriter, req *http.Request) {
	fatal("Unexpected HTTP request", req.URL)
}

func (s *benchService) tearDown() {
	if testZone != "" && testProject != "" && testInstance != "" {
		// Delete this VM
		url := fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/zones/%s/instances/%s",
			testProject, testZone, testInstance)
		print("Asking to delete this VM... ", url)
		req, err := http.NewRequest("DELETE", url, bytes.NewReader(nil))
		if err != nil {
			fatal("Invalid request ", err)
		}
		if _, err := s.gcpClient.Do(req); err != nil {
			fatal("Error deleting this VM ", err)
		}
		print("Done! This VM may...")
	}
	os.Exit(0)
}

func main() {
	flag.Parse()
	address := fmt.Sprintf(":%v", bench.ControllerPort)
	mux := http.NewServeMux()
	server := &http.Server{
		Addr:         address,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0 * time.Second,
		Handler:      http.HandlerFunc(serializeHTTP),
	}

	var c bench.Config

	bc, ok := allClients[testClient]
	if !ok {
		fatal("Please set the BENCHMARK_CLIENT client name")
	}
	if testConfigFile == "" {
		fatal("Please set the BENCHMARK_CONFIG_FILE filename")
	}
	cdata, err := ioutil.ReadFile(testConfigFile)
	if err != nil {
		fatal("Error reading ", testConfigFile, ": ", err.Error())
	}
	err = json.Unmarshal(cdata, &c)
	if err != nil {
		fatal("Error JSON-parsing ", testConfigFile, ": ", err.Error())
	}
	fmt.Println("Config:", string(cdata))

	ctx := context.Background()
	gcpClient, err := google.DefaultClient(ctx, storage.ScopeFullControl)
	if err != nil {
		fatal("GCP Default client: ", err)
	}
	storageClient, err := storage.NewClient(ctx, cloud.WithBaseHTTP(gcpClient))
	if err != nil {
		fatal("GCP Storage client", err)
	}
	defer storageClient.Close()

	service := &benchService{}
	service.processor = lst.NewReportingServiceProcessor(service)
	service.resultCh = make(chan *bench.Result)
	service.controlCh = make(chan *bench.Control)
	service.storage = storageClient
	service.gcpClient = gcpClient
	service.bucket = storageClient.Bucket(testStorageBucket)

	// Test the storage service, auth, etc.
	service.writeTo("test-empty", []byte{})

	go func() {
		for req := range requestCh {
			mux.ServeHTTP(req.w, req.r)
			close(req.doneCh)
		}
	}()

	tfactories := bench.ThriftFactories{
		thrift.NewTProcessorFactory(service.processor),
		thrift.NewTBinaryProtocolFactoryDefault(),
		service}

	mux.HandleFunc(collectorBinaryPath, tfactories.ServeThriftHTTP)
	mux.HandleFunc(collectorJSONPath, service.ServeJSONHTTP)
	mux.HandleFunc(bench.ControlPath, service.ServeControlHTTP)
	mux.HandleFunc(bench.ResultPath, service.ServeResultHTTP)
	mux.HandleFunc("/", service.ServeDefaultHTTP)

	go runGrpc(service)

	go service.runTests(bc, c)

	fatal(server.ListenAndServe())
}

type grpcService struct {
	service *benchService
}

func (g *grpcService) Report(ctx context.Context, req *cpb.ReportRequest) (resp *cpb.ReportResponse, err error) {
	g.service.current.spansReceived += int64(len(req.Spans))
	g.service.countGrpcDroppedSpans(req)
	return
}

func (s *benchService) grpcShim() *grpcService {
	return &grpcService{s}
}

func runGrpc(service *benchService) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", bench.GrpcPort))
	if err != nil {
		fatal("failed to listen:", err)
	}
	grpcServer := grpc.NewServer()

	cpb.RegisterCollectorServiceServer(grpcServer, service.grpcShim())
	fatal(grpcServer.Serve(lis))
}
