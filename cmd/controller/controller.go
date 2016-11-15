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
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"time"

	"cloud.google.com/go/storage"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/grpc"

	bench "github.com/lightstep/lightstep-benchmarks/benchlib"

	proto_timestamp "github.com/golang/protobuf/ptypes/timestamp"
	cpb "github.com/lightstep/lightstep-tracer-go/collectorpb"
	lst "github.com/lightstep/lightstep-tracer-go/lightstep_thrift"
	"github.com/lightstep/lightstep-tracer-go/thrift_0_9_2/lib/go/thrift"
)

const (
	// collectorBinaryPath is the path of the Thrift collector service
	collectorBinaryPath = "/_rpc/v1/reports/binary"
	// collectorJSONPath is the path of the pure-JSON collector service
	collectorJSONPath = "/api/v0/reports"

	nanosPerSecond = 1e9
)

type Duration time.Duration

func (d *Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(*d).String())
}

func (d *Duration) UnmarshalJSON(data []byte) error {
	s := ""
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if pd, err := time.ParseDuration(s); err != nil {
		return err
	} else {
		*d = Duration(pd)
		return nil
	}
}

func (d Duration) Seconds() float64 {
	return time.Duration(d).Seconds()
}

func (d Duration) String() string {
	return time.Duration(d).String()
}

const (
	extraVerbose = false
	warmupRatio  = 0.1
)

type Params struct {
	CalibrateRounds                int
	ExperimentDuration             Duration
	ExperimentRounds               int
	LoadIncrements                 int
	MaximumLoad                    float64
	MaximumRate                    int
	MinimumCalibrations            int
	MaximumCalibrations            int
	MinimumLoad                    float64
	MinimumRate                    int
	NegativeRecalibrationThreshold float64
	RateIncrements                 int
	TestTimeSlice                  Duration
	TestTolerance                  float64

	SleepTrialCount     int
	SleepRepeats        int64
	SleepMinWorkFactor  int64
	SleepMaxWorkFactor  int64
	SleepWorkFactorIncr int64

	SysInterferenceThreshold  float64
	UserInterferenceThreshold float64
}

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
	beforeSelf  bench.Timing
	beforeStat  bench.CPUStat

	// test parameters
	Params

	// Current test results
	benchClient

	// The cost of a single unit of work.
	workCost bench.Timing

	// Cost of tracing a span that does no work.
	spanCost bench.Timing

	spansReceived int64
	spansDropped  int64
	bytesReceived int64
	interferences int
	calibrations  int

	// Process identifier
	pid int
}

type benchClient struct {
	Args []string
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
	s.spansReceived += int64(len(request.SpanRecords))
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
				s.spansDropped += *c.Int64Value
			} else if c.DoubleValue != nil {
				s.spansDropped += int64(*c.DoubleValue)
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
				s.spansDropped += t.IntValue
			case *cpb.MetricsSample_DoubleValue:
				s.spansDropped += int64(t.DoubleValue)
			}
		}
	}
}

// BytesReceived is called from the HTTP layer before Thrift
// processing, recording inbound byte count.
func (s *benchService) BytesReceived(num int64) {
	s.bytesReceived += num
}

// measureSpanCost runs a closed loop creating a certain
// number of spans as quickly as possible and reporting
// the timing.
func (s *benchService) measureSpanCost() {
	s.spanCost = s.measureTestLoop(true)
	bench.Print("Cost T =", s.spanCost, "/span")
}

// estimateWorkCosts measures the cost of the work function.
// TODO this body is now nearly identical to measureTestLoop; Fix.
func (s *benchService) estimateWorkCost() {
	// The work function is assumed to be fast. Find a multiplier
	// that results in working at least bench.TestTimeSlice.
	multiplier := int64(1000000)
	for {
		bench.Print("Testing work for rounds=", multiplier)
		tm := s.run(&bench.Control{
			Concurrent: 1,
			Work:       multiplier,
			Repeat:     1,
		})
		if tm.Measured.User.Seconds() < s.TestTimeSlice.Seconds() {
			multiplier *= 10
			continue
		}
		var st bench.TimingStats
		warmup := int(float64(s.CalibrateRounds) * warmupRatio)
		for j := 0; j < s.CalibrateRounds+warmup; j++ {
			tm := s.run(&bench.Control{
				Concurrent: 1,
				Work:       multiplier,
				Repeat:     1,
			})
			if j < warmup {
				continue
			}
			adjusted := tm.Measured
			st.Update(adjusted)
			if extraVerbose {
				bench.Print("Measured work for rounds", multiplier, "in", adjusted,
					"==", time.Duration(float64(adjusted.User)/float64(multiplier)*1e9))
			}
		}
		s.workCost = st.Mean().Div(float64(multiplier))
		bench.Print("Cost W =", s.workCost, "/unit")
		return
	}
}

func (s *benchService) sanityCheckWork() bool {
	var st bench.TimingStats
	for i := 0; i < s.CalibrateRounds; i++ {
		work := int64(s.TestTimeSlice.Seconds() / s.workCost.User.Seconds())
		tm := s.run(&bench.Control{
			Concurrent: 1,
			Work:       work,
			Repeat:     1,
		})
		adjusted := tm.Measured
		st.Update(adjusted)
	}
	bench.Print("Check work timing", st, "expected", s.TestTimeSlice)

	absRatio := math.Abs((st.User.Mean() - s.TestTimeSlice.Seconds()) / s.TestTimeSlice.Seconds())
	if absRatio > s.TestTolerance {
		fmt.Println("WARNING: CPU work not well calibrated (or insufficient CPU): measured ",
			st.Mean(), " expected ", s.TestTimeSlice,
			" off by ", absRatio*100.0, "%")
		return false
	}
	return true
}

func (s *benchService) measureTestLoop(trace bool) bench.Timing {
	multiplier := int64(1000000)
	for {
		bench.Print("Measuring loop for rounds=", multiplier)
		tm := s.run(&bench.Control{
			Concurrent: 1,
			Work:       0,
			Repeat:     multiplier,
			Trace:      trace,
		})
		if tm.Measured.User.Seconds() < s.TestTimeSlice.Seconds() {
			multiplier *= 10
			continue
		}
		var ss bench.TimingStats
		for j := 0; j < s.CalibrateRounds; j++ {
			tm := s.run(&bench.Control{
				Concurrent: 1,
				Work:       0,
				Repeat:     multiplier,
				Trace:      trace,
			})
			adjusted := tm.Measured
			ss.Update(adjusted)
			if extraVerbose {
				bench.Print("Measured cost for rounds", multiplier, "in", adjusted,
					"==", time.Duration(float64(adjusted.User)/float64(multiplier)*1e9))
			}
		}
		return ss.Mean().Div(float64(multiplier))
	}
}

func (s *benchService) measureSpanImpairment(opts impairmentTest) (bench.DataPoint, float64) {
	qpsPerCpu := opts.rate / float64(opts.concurrency)

	workTime := bench.Time(opts.load / qpsPerCpu)
	sleepTime := bench.Time((1 - opts.load) / qpsPerCpu)
	totalSpans := opts.rate * s.ExperimentDuration.Seconds()
	totalPerCpu := s.ExperimentDuration.Seconds() * qpsPerCpu

	tr := "untraced"
	if opts.trace {
		tr = "traced"
	}
	runOnce := func() (runtime *bench.Timing, spans, dropped, bytes int64, rate, work, sleep float64) {
		sbefore := s.spansReceived
		bbefore := s.bytesReceived
		dbefore := s.spansDropped
		tm := s.run(&bench.Control{
			Concurrent:  opts.concurrency,
			Work:        int64(workTime / s.workCost.User),
			Sleep:       time.Duration(sleepTime * nanosPerSecond),
			Repeat:      int64(totalPerCpu),
			Trace:       opts.trace,
			NumLogs:     opts.lognum,
			BytesPerLog: opts.logsize,
		})
		stotal := s.spansReceived - sbefore
		btotal := s.bytesReceived - bbefore
		dtotal := s.spansDropped - dbefore

		sleepPerCpu := tm.Sleeps.Seconds() / float64(opts.concurrency)
		workPerCpu := totalPerCpu * workTime.Seconds()

		totalTime := tm.Measured.User.Seconds() + tm.Measured.Sys.Seconds() + tm.Sleeps.Seconds()
		totalTimePerCpu := totalTime / float64(opts.concurrency)
		actualRate := totalSpans / totalTimePerCpu

		traceCostPerCpu := totalTimePerCpu - workPerCpu - sleepPerCpu

		impairment := traceCostPerCpu / totalTimePerCpu
		workLoad := workPerCpu / totalTimePerCpu
		sleepLoad := sleepPerCpu / totalTimePerCpu
		visibleLoad := (totalTimePerCpu - sleepPerCpu) / totalTimePerCpu

		// bench.Print("tPc", totalTimePerCpu, "wPc", workPerCpu, "sPc", sleepPerCpu,
		//             "user", tm.Measured.User.Seconds(), "sys", tm.Measured.Sys.Seconds(),
		//             "slept", tm.Sleeps.Seconds())

		completeFrac := 0.0
		if opts.trace && stotal+dtotal != 0 {
			completeFrac = 100 * float64(stotal) / float64(stotal+dtotal)
		}
		bench.Print(fmt.Sprintf("Trial %v@%3f%% %v (log%d*%d,%s) work %.2f%% load %.2f%% impairment %.2f%%, rate %.1f [%0.1f%%]",
			opts.rate, 100*opts.load, totalTimePerCpu, opts.lognum, opts.logsize, tr,
			100*workLoad, 100*visibleLoad, 100*impairment, actualRate, completeFrac))

		// If too far under and not tracing, recalibrate
		if !opts.trace && impairment < s.NegativeRecalibrationThreshold {
			return nil, 0, 0, 0, 0, 0, 0
		}

		return &tm.Measured, stotal, dtotal, btotal, actualRate, workLoad, sleepLoad
	}
	for {
		ss, spans, dropped, _, actualRate, workLoad, sleepLoad := runOnce()

		if ss == nil {
			s.recalibrate()
			continue
		}
		if opts.trace && spans+dropped != int64(totalSpans) {
			bench.Print("Dropped/received spans mismatch", spans, "+", dropped, "!=", totalSpans)
		}

		completeRatio := float64(spans) / totalSpans
		return bench.DataPoint{actualRate, workLoad, sleepLoad}, completeRatio
	}
}

func (s *benchService) measureImpairment(c bench.Config, output *bench.Output) {
	rateInterval := float64(s.MaximumRate-s.MinimumRate) / float64(s.RateIncrements)

	for rate := float64(s.MinimumRate); rate <= float64(s.MaximumRate); rate += rateInterval {
		loadInterval := float64(s.MaximumLoad-s.MinimumLoad) / float64(s.LoadIncrements)
		for load := s.MinimumLoad; load <= float64(s.MaximumLoad); load += loadInterval {
			m := s.measureImpairmentAtRateAndLoad(c, rate, load)
			output.Results = append(output.Results, m...)
		}
	}
}

func (s *benchService) saveResult(result bench.Output) {
	encoded, err := json.MarshalIndent(result, "", "")
	if err != nil {
		bench.Fatal("Couldn't encode JSON!", err)
	}
	withNewline := append(encoded, '\n')
	bench.Print(string(withNewline))
	s.writeTo(path.Join(result.Title, result.Name, result.Client), withNewline)
}

func (s *benchService) writeTo(name string, data []byte) {
	if s.bucket == nil {
		return
	}
	object := s.bucket.Object(name)
	w := object.NewWriter(context.Background())
	_, err := w.Write(data)
	if err != nil {
		bench.Fatal("Couldn't write storage bucket!", err)
	}
	err = w.Close()
	if err != nil {
		bench.Fatal("Couldn't close storage bucket! ", err)
	}
}

func (s *benchService) measureImpairmentAtRateAndLoad(c bench.Config, rate, load float64) []bench.Measurement {
	bench.Print(fmt.Sprintf("Starting rate=%.2f/sec load=%.2f%% test", rate, load*100))
	ms := []bench.Measurement{}
	for i := 0; i < s.ExperimentRounds; i++ {
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
	fmt.Println(fmt.Sprintf("%v: rate=%.2f/sec load=%.2f%% %v", time.Now(), rate, load*100, quickSummary(ms)))
	return ms
}

func quickSummary(ms []bench.Measurement) string {
	var tvr []int64
	var uvr []int64
	var cr []int64
	for _, m := range ms {
		tvr = append(tvr, int64((m.Traced.WorkRatio+m.Traced.SleepRatio)*1e9))
		uvr = append(uvr, int64((m.Untraced.WorkRatio+m.Untraced.SleepRatio)*1e9))
		cr = append(cr, int64(m.Completion*1e9))
	}
	tvl, tvh := bench.Int64NormalConfidenceInterval(tvr)
	uvl, uvh := bench.Int64NormalConfidenceInterval(uvr)
	cm := bench.Int64Mean(cr)
	return fmt.Sprintf("%.2f%% traced [%.3f-%.3f%%] untraced [%.3f-%.3f%%] gap %.3f%%", cm/1e7, tvl/1e7, tvh/1e7, uvl/1e7, uvh/1e7, (uvl-tvh)/1e7)
}

func (s *benchService) estimateSleepCosts(_ bench.Config, o *bench.Output) {
	// Note: the results of this experiment are not used to adjust
	// the impairment measurements.
	bench.Print("Estimating sleep cost")

	type sleepTrial struct {
		with    bench.TimingStats
		without bench.TimingStats
	}

	repeats := s.SleepRepeats
outer:
	for m := s.SleepMinWorkFactor; m <= s.SleepMaxWorkFactor; m += s.SleepWorkFactorIncr {
		equalWork := int64(bench.DefaultSleepInterval.Seconds() / s.workCost.User.Seconds())
		trials := s.SleepTrialCount

		var st sleepTrial

		tci := int(float64(trials) * warmupRatio)
		tc := trials + tci
		for i := 0; i < tc; i++ {
			var ysleep, nsleep *bench.Result
			var s1, s2 time.Duration
			if rand.Float64() < 0.5 {
				s1 = bench.DefaultSleepInterval
			} else {
				s2 = bench.DefaultSleepInterval
			}
			r1 := s.run(&bench.Control{
				Concurrent: 1, // TODO for now..., need to test >1
				Work:       equalWork * m,
				Sleep:      s1,
				Repeat:     repeats,
			})
			r2 := s.run(&bench.Control{
				Concurrent: 1,
				Work:       equalWork * m,
				Sleep:      s2,
				Repeat:     repeats,
			})

			if i < tci {
				continue
			}
			if s2 == 0 {
				ysleep, nsleep = r1, r2
			} else {
				ysleep, nsleep = r2, r1
			}

			st.with.Update(ysleep.Measured)
			st.without.Update(nsleep.Measured)

			o.Sleeps = append(o.Sleeps, bench.SleepCalibration{
				WorkFactor:  int(m),
				RunAndSleep: ysleep.Measured,
				RunNoSleep:  nsleep.Measured,
				Repeats:     int(repeats),
			})
		}

		withLow, _ := st.with.NormalConfidenceInterval()
		_, woHigh := st.without.NormalConfidenceInterval()

		meanTiming := st.with.Mean().Sub(st.without.Mean()).Div(float64(repeats))
		meanCost := meanTiming.User + meanTiming.Sys

		bench.Print(fmt.Sprint("Sleep mean difference: ", meanCost))
		bench.Print(fmt.Sprint("Sleep error separated: ", withLow.Sub(woHigh).Div(float64(repeats))))

		if meanCost < 0 {
			bench.Print("Negative user time: recalibrate:", meanCost)
			s.recalibrate()
			goto outer
		}

		if (st.with.Sys.Mean() < 0 || st.without.Sys.Mean() < 0) && trials < 1000 {
			trials *= 2
			bench.Print("Negative system time: double trials to", trials)
			goto outer
		}
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

	for {
		s.controlCh <- c

		// TODO: Maybe timeout here and help diagnose hung process?
		if r := <-s.resultCh; r != nil {
			return r
		}

		s.interferences++
	}
}

func (s *benchService) runTests(b benchClient, c bench.Config) {
	s.runTest(b, c)
	s.tearDown()
}

func (s *benchService) recalibrate() {
	for s.calibrations < s.MaximumCalibrations {
		if s.calibrations >= s.MinimumCalibrations {
			s.TestTimeSlice *= 2
			if s.TestTimeSlice > s.ExperimentDuration {
				s.TestTimeSlice = s.ExperimentDuration
			}
		}
		bench.Print("Calibration starting, time slice", s.TestTimeSlice, "rounds", s.CalibrateRounds)
		s.calibrations++
		s.spansReceived = 0
		s.spansDropped = 0
		s.bytesReceived = 0
		s.warmup()
		s.estimateWorkCost()
		if !s.sanityCheckWork() {
			continue
		}
		s.measureSpanCost()
		return
	}
}

func (s *benchService) runTest(bc benchClient, c bench.Config) {
	s.benchClient = bc

	bench.Print("Testing ", bench.TestClient)
	ch := make(chan bool)

	defer func() {
		s.exitClient()
		<-ch
	}()

	go s.execClient(bc, ch)

	for s.calibrations < s.MinimumCalibrations {
		s.recalibrate()
	}

	output := bench.Output{}
	output.Title = bench.TestTitle
	output.Client = bench.TestClient
	output.Name = bench.TestConfigName
	output.Concurrent = c.Concurrency
	output.LogBytes = c.LogNum * c.LogSize

	s.estimateSleepCosts(c, &output)
	s.measureImpairment(c, &output)

	s.saveResult(output)
}

func (s *benchService) execClient(bc benchClient, ch chan bool) {
	cmd := exec.Command(bc.Args[0], bc.Args[1:]...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Start(); err != nil {
		bench.Fatal("Could not start client: ", err)
	}
	s.pid = cmd.Process.Pid
	if err := cmd.Wait(); err != nil {
		perr, ok := err.(*exec.ExitError)
		if !ok {
			bench.Fatal("Could not await client: ", err)
		}
		if !perr.Exited() {
			bench.Fatal("Client did not exit: ", err)
		}
		if !perr.Success() {
			bench.Fatal("Client failed: ", string(perr.Stderr))
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
		bench.Fatal("Out-of-phase control request", req.URL)
	}
	s.before, s.beforeSelf, s.beforeStat = bench.GetChildUsage(s.pid)
	s.controlling = true
	body, err := json.Marshal(<-s.controlCh)
	if err != nil {
		bench.Fatal("Marshal error: ", err)
	}
	res.Write(body)
}

// ServeResultHTTP records the client's result via a URL Query parameter "timing".
func (s *benchService) ServeResultHTTP(res http.ResponseWriter, req *http.Request) {
	bres := s.serveResult(req)

	s.controlling = false
	s.resultCh <- bres

	// The response body is not used, but some HTTP clients are
	// troubled by 0-byte responses.
	res.Write([]byte("OK"))
}

func (s *benchService) serveResult(req *http.Request) *bench.Result {
	if !s.controlling {
		bench.Fatal("Out-of-phase client result", req.URL)
	}
	usage, usageSelf, usageStat := bench.GetChildUsage(s.pid)
	usage = usage.Sub(s.before)
	usageSelf = usageSelf.Sub(s.beforeSelf)

	// Note: it would be nice if there were a decoder to unmarshal
	// from URL query param into bench.Result, e.g., opposite of
	// https://godoc.org/github.com/google/go-querystring/query
	params, err := url.ParseQuery(req.URL.RawQuery)
	if err != nil {
		bench.Fatal("Error parsing URL params: ", req.URL.RawQuery)
	}

	// Look for CPU contention on the machine. (TODO 100 == Hz)
	if usage.User.Seconds() > s.Params.TestTimeSlice.Seconds() {
		osUser := bench.Time(float64(usageStat.User-s.beforeStat.User) / 100)
		osSys := bench.Time(float64(usageStat.System-s.beforeStat.System) / 100)

		stolenTicks := usageStat.Steal - s.beforeStat.Steal
		if stolenTicks != 0 {
			bench.Print("Stolen ticks! It's unfair!", stolenTicks)
			return nil
		}

		du := osUser - usage.User - usageSelf.User
		if (du / osUser).Seconds() > s.Params.UserInterferenceThreshold {
			bench.Print(fmt.Sprintf("User interference: %0.1f%% [%.3f/%.3f]", 100*float64(du/osUser), du, usage.User))
			return nil
		}
		ds := osSys - usage.Sys - usageSelf.Sys
		// Compare other system activity against the process's user time
		if (ds / usage.User).Seconds() > s.Params.SysInterferenceThreshold {
			bench.Print(fmt.Sprintf("System interference: %0.1f%% [%.3f/%.3f]", 100*float64(ds/usage.User), ds, usage.User))
			return nil
		}
	}

	return &bench.Result{
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

	s.spansReceived += int64(len(reportRequest.SpanRecords))
	s.bytesReceived += int64(len(body))

	s.countDroppedSpans(reportRequest)

	res.Header().Set("Content-Type", "application/json")
	if err = json.NewEncoder(res).Encode(fakeReportResponse()); err != nil {
		http.Error(res, "Unable to encode response: "+err.Error(), http.StatusBadRequest)
	}
}

func (s *benchService) ServeDefaultHTTP(res http.ResponseWriter, req *http.Request) {
	bench.Fatal("Unexpected HTTP request", req.URL)
}

func (s *benchService) tearDown() {
	if bench.TestZone != "" && bench.TestProject != "" && bench.TestInstance != "" {
		// Delete this VM
		url := fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/zones/%s/instances/%s",
			bench.TestProject, bench.TestZone, bench.TestInstance)
		bench.Print("Asking to delete this VM... ", url)
		req, err := http.NewRequest("DELETE", url, bytes.NewReader(nil))
		if err != nil {
			bench.Fatal("Invalid request ", err)
		}
		if _, err := s.gcpClient.Do(req); err != nil {
			bench.Fatal("Error deleting this VM ", err)
		}
		bench.Print("Done! This VM may...")
	}
	os.Exit(0)
}

func readObject(kind, file, name string, out interface{}) {
	if file == "" {
		bench.Fatal("Please set the", name, "filename")
	}
	cdata, err := ioutil.ReadFile(file)
	if err != nil {
		bench.Fatal("Error reading ", file, ": ", err.Error())
	}
	err = json.Unmarshal(cdata, out)
	if err != nil {
		bench.Fatal("Error JSON-parsing ", file, ": ", err.Error())
	}
	fmt.Println(kind, string(cdata))
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

	bc, ok := allClients[bench.TestClient]
	if !ok {
		bench.Fatal("Please set the BENCHMARK_CLIENT client name")
	}

	service := &benchService{}

	go func() {
		for {
			time.Sleep(10 * time.Minute)
			fmt.Println(time.Now(), ":", service.interferences, "interferences", service.calibrations, "calibrations")
		}
	}()

	service.processor = lst.NewReportingServiceProcessor(service)
	service.resultCh = make(chan *bench.Result)
	service.controlCh = make(chan *bench.Control)

	readObject("Config", bench.TestConfigFile, "BENCHMARK_CONFIG_FILE", &c)
	readObject("Params", bench.TestParamsFile, "BENCHMARK_PARAMS_FILE", &service.Params)

	if service.Params.UserInterferenceThreshold == 0 {
		service.Params.UserInterferenceThreshold = 0.01
	}
	if service.Params.SysInterferenceThreshold == 0 {
		service.Params.SysInterferenceThreshold = 0.02
	}

	var err error
	ctx := context.Background()
	service.gcpClient, err = google.DefaultClient(ctx, storage.ScopeFullControl)
	if err != nil {
		bench.Print("GCP Default client: ", err)
		bench.Print("Will not write results to GCP")
	} else {
		service.storage, err = storage.NewClient(ctx, option.WithHTTPClient(service.gcpClient))
		if err != nil {
			bench.Print("GCP Storage client", err)
		} else {
			defer service.storage.Close()
			service.bucket = service.storage.Bucket(bench.TestStorageBucket)

			// Test the storage service, auth, etc.
			service.writeTo("test-empty", []byte{})
		}
	}
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

	bench.Fatal(server.ListenAndServe())
}

type grpcService struct {
	service *benchService
}

func (g *grpcService) Report(ctx context.Context, req *cpb.ReportRequest) (resp *cpb.ReportResponse, err error) {
	g.service.spansReceived += int64(len(req.Spans))
	g.service.countGrpcDroppedSpans(req)
	now := time.Now()
	ts := &proto_timestamp.Timestamp{
		Seconds: now.Unix(),
		Nanos:   int32(now.Nanosecond()),
	}
	return &cpb.ReportResponse{ReceiveTimestamp: ts, TransmitTimestamp: ts}, nil
}

func (s *benchService) grpcShim() *grpcService {
	return &grpcService{s}
}

func runGrpc(service *benchService) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", bench.GrpcPort))
	if err != nil {
		bench.Fatal("failed to listen:", err)
	}
	grpcServer := grpc.NewServer()

	cpb.RegisterCollectorServiceServer(grpcServer, service.grpcShim())
	bench.Fatal(grpcServer.Serve(lis))
}
