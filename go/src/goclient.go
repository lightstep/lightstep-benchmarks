package main

import (
	"benchlib"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/golang/glog"
	ls "github.com/lightstep/lightstep-tracer-go"
	"github.com/opentracing/basictracer-go"
	ot "github.com/opentracing/opentracing-go"
)

const (
	clientName = "golang"
)

type testClient struct {
	baseURL string
	tracer  ot.Tracer
}

type sleeper struct {
	sleepDebt time.Duration
}

func (s *sleeper) amortizedSleep(d, interval time.Duration) {
	s.sleepDebt += d
	if s.sleepDebt > interval {
		s.sleep()
	}
}

func (s *sleeper) sleep() {
	if s.sleepDebt < 0 {
		return
	}
	begin := time.Now()
	for s.sleepDebt > 0 {
		time.Sleep(s.sleepDebt)
		now := time.Now()
		s.sleepDebt -= now.Sub(begin)
		begin = now
	}
}

func work(n int64) int64 {
	const primeWork = 982451653
	x := int64(primeWork)
	for n >= 0 {
		x *= primeWork
		n--
	}
	return x
}

func (t *testClient) getURL(path string) []byte {
	resp, err := http.Get(t.baseURL + path)
	if err != nil {
		glog.Fatal("Bench control request failed: ", err)
	}
	if resp.StatusCode != 200 {
		glog.Fatal("Bench control status != 200: ", resp.Status)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		glog.Fatal("Bench error reading body: ", err)
	}
	return body
}

func (t *testClient) loop() {
	for {
		body := t.getURL(benchlib.ControlPath)

		control := benchlib.Control{}
		if err := json.Unmarshal(body, &control); err != nil {
			glog.Fatal("Bench control parse error: ", err)
		}
		if control.Exit {
			return
		}
		timing := t.run(&control)

		t.getURL(fmt.Sprint(
			benchlib.ResultPath,
			"?timing=",
			timing.Seconds()))
	}
}

func testBody(control *benchlib.Control) {
	var s sleeper
	for i := int64(0); i < control.Repeat; i++ {
		span := ot.StartSpan("span/test")
		work(control.Work)
		span.Finish()
		if control.Sleep != 0 {
			s.amortizedSleep(control.Sleep, control.SleepInterval)
		}
	}
	s.sleep()
}

func (t *testClient) run(control *benchlib.Control) time.Duration {
	if control.Trace {
		ot.InitGlobalTracer(t.tracer)
	} else {
		ot.InitGlobalTracer(ot.NoopTracer{})
	}
	runtime.GOMAXPROCS(1) // TODO
	runtime.GC()

	conc := control.Concurrent
	beginTest := time.Now()
	if conc == 1 {
		testBody(control)
	} else {
		start := &sync.WaitGroup{}
		finish := &sync.WaitGroup{}
		start.Add(conc)
		finish.Add(conc)
		for c := 0; c < conc; c++ {
			go func() {
				start.Done()
				start.Wait()
				testBody(control)
				finish.Done()
			}()
		}
		finish.Wait()
	}
	if control.Trace && !control.NoFlush {
		recorder := t.tracer.(basictracer.Tracer).Options().Recorder.(*ls.Recorder)
		recorder.Flush()
	}
	return time.Now().Sub(beginTest)
}

func main() {
	flag.Parse()
	tc := &testClient{
		baseURL: fmt.Sprint("http://",
			benchlib.ControllerHost, ":",
			benchlib.ControllerPort),
		tracer: ls.NewTracer(ls.Options{
			AccessToken:        benchlib.ControllerAccessToken,
			CollectorHost:      benchlib.ControllerHost,
			CollectorPort:      benchlib.ControllerPort,
			CollectorPlaintext: true})}
	tc.loop()
}
