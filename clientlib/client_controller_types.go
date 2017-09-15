package clientlib

import (
	"github.com/lightstep/lightstep-benchmarks/benchlib"
	"net/http"
	"time"
)

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
