package clientlib

import (
	"encoding/json"
	"github.com/lightstep/lightstep-benchmarks/benchlib"
	"net/http"
	"time"
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

type (
	TestClientController interface {
		StartControlServer()
		StopControlServer() error

		StartClient(TestClient) error
		Run(Control) (*benchlib.Result, error)
		StopClient()
	}
	TestClient interface {
		Start() error
		WaitForExit()
		Pid() int
	}

	sreq struct {
		w      http.ResponseWriter
		r      *http.Request
		doneCh chan struct{}
	}
	Control struct {
		Concurrent int // How many routines, threads, etc.

		// How much work to perform under one span
		Work int64

		// How many repetitions
		Repeat int64

		// How many amortized nanoseconds to sleep after each span
		Sleep time.Duration
		// How many nanoseconds to sleep at once
		SleepInterval time.Duration

		// How many bytes per log statement
		BytesPerLog int64
		NumLogs     int64

		// Misc control bits
		Trace bool // Trace the operation.
		Exit  bool // Terminate the test.
	}
)

var Clients = map[string]TestClient{
	"golang": CreateProcessClient([]string{"./goclient"}),
}
