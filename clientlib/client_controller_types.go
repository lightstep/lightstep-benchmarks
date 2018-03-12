package clientlib

import (
	"github.com/lightstep/lightstep-benchmarks/bench"
)

type (
	TestClientController interface {
		StartControlServer()
		StopControlServer() error

		StartClient(TestClient) error
		Run(Control) (Result, error)
		StopClient()
	}

	TestClient interface {
		Exec()
		Wait() error
		ProcessID() int
	}

	Result  = bench.Result
	Control = bench.Control
)
