package clientlib

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/lightstep/lightstep-benchmarks/bench"
)

const (
	defaultUserInterferenceThreshold = 0.01
	defaultSysInterferenceThreshold  = 0.02

	ControlPath    = "/control"
	ResultPath     = "/result"
	ControllerPort = 8000

	stateControl  controllerState = 0
	stateResponse                 = iota
)

type (
	// HTTPTestClientController executes a test via control instructions to a client.
	// (was extracted from `benchService`).
	HTTPTestClientController struct {
		controlCh chan Control
		resultCh  chan *Result
		requestCh chan sreq

		server *http.Server

		// Params
		TestTimeSlice             Duration
		UserInterferenceThreshold float64
		SysInterferenceThreshold  float64

		// Client
		client TestClient
		state  controllerState

		// Client stats
		before      bench.Timing
		beforeSelf  bench.Timing
		beforeStat  bench.CPUStat
		repetitions int
	}

	sreq struct {
		w      http.ResponseWriter
		r      *http.Request
		doneCh chan struct{}
	}

	// controllerState: whether to expect a control request or result response.
	controllerState int
)

func CreateHTTPTestClientController() TestClientController {
	return &HTTPTestClientController{}
}

func (c *HTTPTestClientController) serializeHTTP(w http.ResponseWriter, r *http.Request) {
	doneCh := make(chan struct{})
	c.requestCh <- sreq{w, r, doneCh}
	<-doneCh
}

func (c *HTTPTestClientController) StartClient(client TestClient) error {
	c.client = client
	return c.client.Start()
}

func (c *HTTPTestClientController) StopClient() {
	c.controlCh <- Control{Exit: true}
	c.client.WaitForExit()
	c.state = stateControl
}

// Runs executes the Control repeatedly until receiving a successful response.
func (c *HTTPTestClientController) Run(control Control) (Result, error) {

	if control.SleepInterval == 0 {
		// @@@ where does this belong
		control.SleepInterval = bench.DefaultSleepInterval
	}

	for {
		c.controlCh <- control

		// TODO: Maybe timeout here and help diagnose hung process?
		if r := <-c.resultCh; r != nil {
			return *r, nil
		}

		// formResult() returns nil when there are errors, CPU
		// contention, etc., so that the test will be repeated.
		c.repetitions++
	}
}

func (c *HTTPTestClientController) StartControlServer() {
	address := fmt.Sprintf(":%v", ControllerPort)
	mux := http.NewServeMux()
	// Note: the 100000 second timeout avoids HTTP disconnections,
	// which can confuse very simple HTTP libraries (e.g., the C++
	// benchmark client).
	c.server = &http.Server{
		Addr:         address,
		ReadTimeout:  100000 * time.Second,
		WriteTimeout: 0 * time.Second,
		Handler:      http.HandlerFunc(c.serializeHTTP),
	}

	c.resultCh = make(chan *Result)
	c.controlCh = make(chan Control)
	c.requestCh = make(chan sreq)

	if c.UserInterferenceThreshold == 0 {
		c.UserInterferenceThreshold = defaultUserInterferenceThreshold
	}
	if c.SysInterferenceThreshold == 0 {
		c.SysInterferenceThreshold = defaultSysInterferenceThreshold
	}
	mux.HandleFunc(ControlPath, c.serveControlHTTP)
	mux.HandleFunc(ResultPath, c.serveResultHTTP)
	mux.HandleFunc("/", c.serveDefaultHTTP)

	go func() {
		for req := range c.requestCh {
			mux.ServeHTTP(req.w, req.r)
			close(req.doneCh)
		}
	}()

	go func() {
		err := c.server.ListenAndServe()
		if err != http.ErrServerClosed {
			panic(err)
		}
	}()
}

func (c *HTTPTestClientController) StopControlServer() error {
	close(c.resultCh)
	close(c.controlCh)
	close(c.requestCh)

	ctx := context.TODO()
	return c.server.Shutdown(ctx)
}

func (c *HTTPTestClientController) serveDefaultHTTP(res http.ResponseWriter, req *http.Request) {
	panic(fmt.Errorf("Unexpected HTTP request: %v", req.URL))
}

func (c *HTTPTestClientController) serveControlHTTP(res http.ResponseWriter, req *http.Request) {
	if c.state != stateControl {
		panic(fmt.Errorf("Out-of-phase control request: %v", req.URL))
	}

	c.before, c.beforeSelf, c.beforeStat = bench.GetChildUsage(c.client.Pid())
	c.state = stateResponse
	control := <-c.controlCh
	body, err := json.Marshal(control)
	if err != nil {
		panic(fmt.Errorf("Marshal error: %v", err))
	}
	_, err = res.Write(body)
	if err != nil {
		panic(fmt.Errorf("Response write error: %v", err))
	}
}

func (c *HTTPTestClientController) serveResultHTTP(res http.ResponseWriter, req *http.Request) {
	if c.state != stateResponse {
		panic(fmt.Errorf("Out-of-phase client result: %v", req.URL))
	}

	benchResult := c.formResult(req)
	c.state = stateControl
	c.resultCh <- benchResult

	// The response body is not used, but some HTTP clients are
	// troubled by 0-byte responses.
	_, err := res.Write([]byte("OK"))
	if err != nil {
		panic(fmt.Errorf("Response write error: %v", err))
	}
}

// formResult completes the test measurement, filling in the observed
// sys/user CPU usage and returning the result struct.
func (c *HTTPTestClientController) formResult(req *http.Request) *Result {
	usage, usageSelf, usageStat := bench.GetChildUsage(c.client.Pid())
	usage = usage.Sub(c.before)
	usageSelf = usageSelf.Sub(c.beforeSelf)

	// Note: it would be nice if there were a decoder to unmarshal
	// from URL query param into Result, e.g., opposite of
	// https://godoc.org/github.com/google/go-querystring/query
	params, err := url.ParseQuery(req.URL.RawQuery)
	if err != nil {
		panic(fmt.Errorf("Error parsing URL params: %v", req.URL.RawQuery))
	}

	// Look for CPU contention on the machine. (TODO 100 == Hz)
	if usage.User.Seconds() > c.TestTimeSlice.Seconds() {
		osUser := bench.Time(float64(usageStat.User-c.beforeStat.User) / 100)
		osSys := bench.Time(float64(usageStat.System-c.beforeStat.System) / 100)

		stolenTicks := usageStat.Steal - c.beforeStat.Steal
		if stolenTicks != 0 {
			fmt.Println("Stolen ticks! It's unfair!", stolenTicks)
			return nil
		}

		du := osUser - usage.User - usageSelf.User
		if (du / osUser).Seconds() > c.UserInterferenceThreshold {
			fmt.Println(fmt.Sprintf("User interference: %0.1f%% [%.3f/%.3f]", 100*float64(du/osUser), du, usage.User))
			return nil
		}
		ds := osSys - usage.Sys - usageSelf.Sys
		// Compare other system activity against the process's user time
		if (ds / usage.User).Seconds() > c.SysInterferenceThreshold {
			fmt.Printf("System interference: %0.1f%% [%.3f/%.3f]", 100*float64(ds/usage.User), ds, usage.User)
			return nil
		}
	}

	return &Result{
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
