package clientlib

import (
	"github.com/lightstep/lightstep-benchmarks/benchlib"

	"fmt"
	"testing"
)

func TestClientStart(t *testing.T) {
	controller := CreateHTTPTestClientController()
	fmt.Printf("Starting Control server")
	controller.StartControlServer()
	fmt.Printf("Control server started")

	client := CreateProcessClient([]string{"./goclient"})
	err := controller.StartClient(client)
	if err != nil {
		t.Error("Should not have errored")
	}

	// Run warmup tasks
	res, err := controller.Run(benchlib.Control{
		Concurrent:    1,
		Work:          1000,
		Repeat:        10,
		Trace:         false,
		Sleep:         1,
		SleepInterval: 5,
	})
	if err != nil {
		t.Error("Should not have errored")
	}
	fmt.Println(res)
	if res == nil {
		t.Error("Should have returned a result")
	}

	res, err = controller.Run(benchlib.Control{
		Concurrent:    1,
		Work:          1000,
		Repeat:        10,
		Trace:         true,
		Sleep:         10,
		SleepInterval: 100,
	})

	if err != nil {
		t.Error("Should not have errored")
	}
	fmt.Println(res)
	if res == nil {
		t.Error("Should have returned a result")
	}

	controller.StopClient()
}
