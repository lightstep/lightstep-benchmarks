package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
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

	bench "benchlib"

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

	// minimumCalibrations = 1
	// calibrateRounds     = 2000

	minimumCalibrations = 1
	calibrateRounds     = 200

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
	sleeptrialcount     = 20
	sleeprepeats        = int64(20)
	sleepmaxworkfactor  = int64(100)
	sleepminworkfactor  = int64(10)
	sleepworkfactorincr = int64(30)
)

var (
	// client is a list of client programs for the benchmark
	allclients = map[string]benchclient{
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
			// "-xdebug", "-xrunjdwp:transport=dt_socket,address=7000,server=y,suspend=n",
			"com.lightstep.benchmark.benchmarkclient"}},
	}

	// requestch is used to serialize http requests
	requestch = make(chan sreq)

	teststoragebucket = getenv("benchmark_bucket", "lightstep-client-benchmarks")
	testtitle         = getenv("benchmark_title", "untitled")
	testconfigname    = getenv("benchmark_config_name", "unnamed")
	testconfigfile    = getenv("benchmark_config_file", "config.json")
	testclient        = getenv("benchmark_client", "unknown")
	testzone          = getenv("benchmark_zone", "")
	testproject       = getenv("benchmark_project", "")
	testinstance      = getenv("benchmark_instance", "")
	testverbose       = getenv("benchmark_verbose", "false")
)

type impairmenttest struct {
	// configuration
	concurrency int
	lognum      int64
	logsize     int64

	// experiment variables
	trace bool
	rate  float64
	load  float64
}

type sreq struct {
	w      http.responsewriter
	r      *http.request
	donech chan struct{}
}

type benchservice struct {
	processor        *lst.reportingserviceprocessor
	processorfactory thrift.tprocessorfactory
	protocolfactory  thrift.tprotocolfactory
	controlch        chan *bench.control
	resultch         chan *bench.result
	storage          *storage.client
	bucket           *storage.buckethandle
	gcpclient        *http.client

	// outstanding request state
	controlling bool
	before      bench.timing

	// current collects results for the current test
	current *benchstats
}

type benchstats struct {
	benchclient

	// number of times calibration has been performed.
	calibrations int

	// the cost of doing zero repetitions, indexed by concurrency.
	// note: this is a small, sparse array because we only test
	// power-of-two configurations.
	zerocost []bench.timing

	// the cost of a round w/ no working, no sleeping, no tracing.
	roundcost bench.timing

	// the cost of a single unit of work.
	workcost bench.timing

	// cost of tracing a span that does no work.
	spancost bench.timing

	spansreceived int64
	spansdropped  int64
	bytesreceived int64
}

type benchclient struct {
	args []string
}

func fatal(x ...interface{}) {
	panic(fmt.sprint(x...))
}

func print(x ...interface{}) {
	if testverbose == "true" {
		fmt.sprintln(x...)
	}
}

func getenv(name, defval string) string {
	if r := os.getenv(name); r != "" {
		return r
	}
	return defval
}

func newbenchstats(bc benchclient) *benchstats {
	return &benchstats{
		benchclient: bc,
		zerocost:    make([]bench.timing, maxconcurrency+1, maxconcurrency+1),
	}
}

func serializehttp(w http.responsewriter, r *http.request) {
	donech := make(chan struct{})
	requestch <- sreq{w, r, donech}
	<-donech
}

func fakereportresponse() *lst.reportresponse {
	nowmicros := time.now().unixnano() / 1000
	return &lst.reportresponse{timing: &lst.timing{&nowmicros, &nowmicros}}
}

// report is a thrift collector method.
func (s *benchservice) report(auth *lst.auth, request *lst.reportrequest) (
	r *lst.reportresponse, err error) {
	s.current.spansreceived += int64(len(request.spanrecords))
	s.countdroppedspans(request)
	return fakereportresponse(), nil
}

func (s *benchservice) countdroppedspans(request *lst.reportrequest) {
	if request.internalmetrics == nil {
		return
	}
	for _, c := range request.internalmetrics.counts {
		if c.name == "spans.dropped" {
			if c.int64value != nil {
				s.current.spansdropped += *c.int64value
			} else if c.doublevalue != nil {
				s.current.spansdropped += int64(*c.doublevalue)
			}
		}
	}
}

// bytesreceived is called from the http layer before thrift
// processing, recording inbound byte count.
func (s *benchservice) bytesreceived(num int64) {
	s.current.bytesreceived += num
}

// estimatezerocosts measures the cost of doing nothing.
func (s *benchservice) estimatezerocosts() {
	for c := 1; c <= maxconcurrency; c *= 2 {
		var st bench.timingstats
		for j := 0; j < testiteration; j++ {
			tm := s.run(&bench.control{
				concurrent: c,
			})
			st.update(tm.measured)
		}
		s.current.zerocost[c] = st.mean()
		print("cost z_c_", c, " = ", s.current.zerocost[c])
	}
}

// measurespancost runs a closed loop creating a certain
// number of spans as quickly as possible and reporting
// the timing.
func (s *benchservice) measurespancost() {
	s.current.spancost = s.measuretestloop(true)
	print("cost t = %s/span", s.current.spancost)
}

// estimateroundcost runs a untraced loop doing no work to establish
// the baseline cost of a repetition.
func (s *benchservice) estimateroundcost() {
	s.current.roundcost = s.measuretestloop(false)
	print("cost r = %s/round", s.current.roundcost)
}

// estimateworkcosts measures the cost of the work function.
// todo this body is now nearly identical to measuretestloop; fix.
func (s *benchservice) estimateworkcost() {
	// the work function is assumed to be fast. find a multiplier
	// that results in working at least testtimeslice.
	multiplier := int64(1000)
	for {
		print("testing work for rounds=", multiplier)
		tm := s.run(&bench.control{
			concurrent: 1,
			work:       multiplier,
			repeat:     1,
		})
		if tm.measured.wall.seconds() < testtimeslice.seconds() {
			multiplier *= 10
			continue
		}
		var st bench.timingstats
		for j := 0; j < calibraterounds; j++ {
			print("measuring work for rounds=", multiplier)
			tm := s.run(&bench.control{
				concurrent: 1,
				work:       multiplier,
				repeat:     1,
			})
			adjusted := tm.measured.sub(s.current.zerocost[1]).sub(s.current.roundcost)
			st.update(adjusted)
			print("measured work for rounds=", multiplier, " in ", adjusted,
				" == ", float64(adjusted.wall)/float64(multiplier))
		}
		s.current.workcost = st.mean().div(float64(multiplier))
		print("cost w = %s/unit", s.current.workcost)
		return
	}
}

func (s *benchservice) sanitycheckwork() bool {
	var st bench.timingstats
	for i := 0; i < calibraterounds; i++ {
		work := int64(testtimeslice.seconds() / s.current.workcost.wall.seconds())
		tm := s.run(&bench.control{
			concurrent: 1,
			work:       work,
			repeat:     1,
		})
		adjusted := tm.measured.sub(s.current.zerocost[1]).sub(s.current.roundcost)
		st.update(adjusted)
	}
	print("check work timing", st, "expected", testtimeslice)

	absratio := math.abs((st.wall.mean() - testtimeslice.seconds()) / testtimeslice.seconds())
	if absratio > testtolerance {
		fmt.print(fmt.sprint("warning: cpu work not well calibrated (or insufficient cpu): measured ",
			st.mean(), " expected ", testtimeslice,
			" off by ", absratio*100.0, "%\n"))
		return false
	}
	return true
}

func (s *benchservice) measuretestloop(trace bool) bench.timing {
	multiplier := int64(1000)
	for {
		print("measuring loop for rounds=", multiplier)
		tm := s.run(&bench.control{
			concurrent: 1,
			work:       0,
			repeat:     multiplier,
			trace:      trace,
		})
		if tm.measured.wall.seconds() < testtimeslice.seconds() {
			multiplier *= 10
			continue
		}
		var ss bench.timingstats
		for j := 0; j < calibraterounds; j++ {
			tm := s.run(&bench.control{
				concurrent: 1,
				work:       0,
				repeat:     multiplier,
				trace:      trace,
			})
			adjusted := tm.measured.sub(s.current.zerocost[1])
			if trace {
				adjusted = adjusted.subfactor(s.current.roundcost, float64(multiplier))
			}
			ss.update(adjusted)
			print("measured cost for rounds=", multiplier, " in ", adjusted,
				" == ", float64(adjusted.wall)/float64(multiplier))
		}
		return ss.mean().div(float64(multiplier))
	}
}

func (s *benchservice) measurespanimpairment(opts impairmenttest) (bench.datapoint, float64) {
	qpspercpu := opts.rate / float64(opts.concurrency)

	worktime := bench.time(opts.load / qpspercpu)
	sleeptime := bench.time((1 - opts.load) / qpspercpu)
	totalspans := opts.rate * experimentduration
	totalpercpu := experimentduration * qpspercpu

	tr := "untraced"
	if opts.trace {
		tr = "traced"
	}
	runonce := func() (runtime *bench.timing, spans, dropped, bytes int64, rate, work, sleep float64) {
		sbefore := s.current.spansreceived
		bbefore := s.current.bytesreceived
		dbefore := s.current.spansdropped
		tm := s.run(&bench.control{
			concurrent:  opts.concurrency,
			work:        int64(worktime / s.current.workcost.wall),
			sleep:       time.duration(sleeptime * nanospersecond),
			repeat:      int64(totalpercpu),
			trace:       opts.trace,
			numlogs:     opts.lognum,
			bytesperlog: opts.logsize,
		})
		stotal := s.current.spansreceived - sbefore
		btotal := s.current.bytesreceived - bbefore
		dtotal := s.current.spansdropped - dbefore

		adjusted := tm.measured.sub(s.current.zerocost[opts.concurrency]).subfactor(s.current.roundcost, totalpercpu)
		sleeppercpu := tm.sleeps.seconds() / float64(opts.concurrency)
		workpercpu := totalpercpu * worktime.seconds()
		actualrate := totalspans / adjusted.wall.seconds()
		tracecost := adjusted.wall.seconds() - workpercpu - sleeppercpu
		impairment := tracecost / adjusted.wall.seconds()
		workload := workpercpu / adjusted.wall.seconds()
		sleepload := sleeppercpu / adjusted.wall.seconds()
		visibleload := (adjusted.wall.seconds() - sleeppercpu) / adjusted.wall.seconds()

		print("trial %v@%3f%% %v (log%d*%d,%s) work load %.2f%% visible load %.2f%% visible impairment %.2f%%, actual rate %.1f",
			opts.rate, 100*opts.load, adjusted.wall, opts.lognum, opts.logsize, tr,
			100*workload, 100*visibleload, 100*impairment, actualrate)

		// if too far under, recalibrate
		if impairment < negativerecalibrationthreshold {
			return nil, 0, 0, 0, 0, 0, 0
		}

		return &adjusted, stotal, dtotal, btotal, actualrate, workload, sleepload
	}
	for {
		if s.current.calibrations < minimumcalibrations {
			// adjust for on-the-fly compilation,
			// initialization costs, etc.
			s.recalibrate()
		}

		ss, spans, _, _, actualrate, workload, sleepload := runonce()

		if ss == nil {
			s.recalibrate()
			continue
		}

		completeratio := float64(spans) / totalspans
		return bench.datapoint{actualrate, workload, sleepload}, completeratio
	}
}

func (s *benchservice) measureimpairment(c bench.config, output *bench.output) {
	rateinterval := float64(maximumrate-minimumrate) / rateincrements

	for rate := float64(minimumrate); rate <= maximumrate; rate += rateinterval {
		loadinterval := float64(maximumload-minimumload) / loadincrements
		for load := minimumload; load <= maximumload; load += loadinterval {
			m := s.measureimpairmentatrateandload(c, rate, load)
			output.results = append(output.results, m...)
		}
	}
}

func (s *benchservice) saveresult(result bench.output) {
	encoded, err := json.marshalindent(result, "", "")
	if err != nil {
		fatal("couldn't encode json! " + err.error())
	}
	withnewline := append(encoded, '\n')
	fmt.print(string(withnewline))
	s.writeto(path.join(result.title, result.name, result.client), withnewline)
}

func (s *benchservice) writeto(name string, data []byte) {
	object := s.bucket.object(name)
	w := object.newwriter(context.background())
	_, err := w.write(data)
	if err != nil {
		fatal("couldn't write storage bucket! " + err.error())
	}
	err = w.close()
	if err != nil {
		fatal("couldn't close storage bucket! " + err.error())
	}
}

func (s *benchservice) measureimpairmentatrateandload(c bench.config, rate, load float64) []bench.measurement {
	print("starting rate=%.2f/sec load=%.2f%% test", rate, load*100)
	ms := []bench.measurement{}
	for i := 0; i < experimentrounds; i++ {
		m := bench.measurement{}
		m.targetrate = rate
		m.targetload = load
		m.untraced, _ = s.measurespanimpairment(impairmenttest{
			trace:       false,
			concurrency: c.concurrency,
			rate:        rate,
			load:        load,
			lognum:      c.lognum,
			logsize:     c.logsize,
		})
		m.traced, m.completion = s.measurespanimpairment(impairmenttest{
			trace:       true,
			concurrency: c.concurrency,
			rate:        rate,
			load:        load,
			lognum:      c.lognum,
			logsize:     c.logsize,
		})
		ms = append(ms, m)

	}
	return ms
}

func (s *benchservice) estimatesleepcosts(_ bench.config, o *bench.output) {
	print("estimating sleep cost")

	equalwork := int64(bench.defaultsleepinterval.seconds() / s.current.workcost.wall.seconds())

	type sleeptrial struct {
		with    bench.timingstats
		without bench.timingstats
		sleeps  bench.timingstats
	}

	var sleeptrials []sleeptrial

	for m := sleepminworkfactor; m <= sleepmaxworkfactor; m += sleepworkfactorincr {
		var st sleeptrial
		for i := 0; i < sleeptrialcount; i++ { // todo should be ... until 95% confidence or at least n
			wsleep := s.run(&bench.control{
				concurrent: 1, // todo for now..., need to test >1
				work:       equalwork * m,
				sleep:      bench.defaultsleepinterval,
				repeat:     sleeprepeats,
			})
			ssleep := s.run(&bench.control{
				concurrent: 1,
				work:       equalwork * m,
				sleep:      0,
				repeat:     sleeprepeats,
			})

			st.with.update(wsleep.measured)
			st.without.update(ssleep.measured)
			st.sleeps.update(bench.timing{wall: wsleep.sleeps})

			o.sleeps = append(o.sleeps, bench.sleepcalibration{
				workfactor:  m,
				runandsleep: wsleep.measured.wall.seconds(),
				runnosleep:  ssleep.measured.wall.seconds(),
				actualsleep: wsleep.sleeps.seconds(),
				repeats:     sleeprepeats,
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
		Print("Calibration starting")
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

	Print("Testing ", testClient)
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
		Fatal("Could not start client: ", err)
	}
	if err := cmd.Wait(); err != nil {
		perr, ok := err.(*exec.ExitError)
		if !ok {
			Fatal("Could not await client: ", err)
		}
		if !perr.Exited() {
			Fatal("Client did not exit: ", err)
		}
		if !perr.Success() {
			Fatal("Client failed: ", string(perr.Stderr))
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
		Fatal("Out-of-phase control request", req.URL)
	}
	s.before = bench.GetChildUsage()
	s.controlling = true
	body, err := json.Marshal(<-s.controlCh)
	if err != nil {
		Fatal("Marshal error: ", err)
	}
	res.Write(body)
}

// ServeResultHTTP records the client's result via a URL Query parameter "timing".
func (s *benchService) ServeResultHTTP(res http.ResponseWriter, req *http.Request) {
	if !s.controlling {
		Fatal("Out-of-phase client result", req.URL)
	}
	usage := bench.GetChildUsage().Sub(s.before)
	// Note: it would be nice if there were a decoder to unmarshal
	// from URL query param into bench.Result, e.g., opposite of
	// https://godoc.org/github.com/google/go-querystring/query
	params, err := url.ParseQuery(req.URL.RawQuery)

	if err != nil {
		Fatal("Error parsing URL params: ", req.URL.RawQuery)
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
	Fatal("Unexpected HTTP request", req.URL)
}

func (s *benchService) tearDown() {
	if testZone != "" && testProject != "" && testInstance != "" {
		// Delete this VM
		url := fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/zones/%s/instances/%s",
			testProject, testZone, testInstance)
		Print("Asking to delete this VM... ", url)
		req, err := http.NewRequest("DELETE", url, bytes.NewReader(nil))
		if err != nil {
			Fatal("Invalid request ", err)
		}
		if _, err := s.gcpClient.Do(req); err != nil {
			Fatal("Error deleting this VM ", err)
		}
		Print("Done! This VM may...")
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
		Fatal("Please set the BENCHMARK_CLIENT client name")
	}
	if testConfigFile == "" {
		Fatal("Please set the BENCHMARK_CONFIG_FILE filename")
	}
	cdata, err := ioutil.ReadFile(testConfigFile)
	if err != nil {
		Fatal("Error reading ", testConfigFile, ": ", err.Error())
	}
	err = json.Unmarshal(cdata, &c)
	if err != nil {
		Fatal("Error JSON-parsing ", testConfigFile, ": ", err.Error())
	}
	fmt.Print("Config:", string(cdata))

	ctx := context.Background()
	gcpClient, err := google.DefaultClient(ctx, storage.ScopeFullControl)
	if err != nil {
		Fatal("GCP Default client: ", err)
	}
	storageClient, err := storage.NewClient(ctx, cloud.WithBaseHTTP(gcpClient))
	if err != nil {
		log.Fatal("GCP Storage client", err)
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

	go service.runTests(bc, c)

	Fatal(server.ListenAndServe())
}
