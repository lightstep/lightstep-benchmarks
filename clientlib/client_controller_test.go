package clientlib

import (
	"github.com/lightstep/lightstep-benchmarks/benchlib"

	"testing"
)

type TestClientControllerTest struct {
	Control          benchlib.Control
	ExpectedResponse *benchlib.Result
	ExpectedError    error
}

var TestClients = []TestClient{
	CreateProcessClient([]string{"./goclient"}),
}

var TestRuns = []TestClientControllerTest{
	TestClientControllerTest{
		Control: benchlib.Control{
			Concurrent:    1,
			Work:          1000,
			Repeat:        10,
			Trace:         false,
			Sleep:         1,
			SleepInterval: 5,
		},
		ExpectedResponse: &benchlib.Result{},
		ExpectedError:    nil,
	},
	TestClientControllerTest{
		Control: benchlib.Control{
			Concurrent:    1,
			Work:          1000,
			Repeat:        10,
			Trace:         true,
			Sleep:         10,
			SleepInterval: 100,
		},
		ExpectedResponse: &benchlib.Result{},
		ExpectedError:    nil,
	},
}

func TestClientStart(t *testing.T) {
	controller := CreateHTTPTestClientController()
	controller.StartControlServer()

	for _, client := range TestClients {
		err := controller.StartClient(client)
		if err != nil {
			t.Error("Client %v failed to start with error: %v", client, err)
		}

		for _, test := range TestRuns {
			// Run warmup tasks
			res, err := controller.Run(test.Control)
			if test.ExpectedError == nil && err != nil {
				t.Error("Run with control: %v unexpectedly returned an error: %v", test.Control, err)
			}

			if test.ExpectedError != nil && err == nil {
				t.Error("Run with control: %v expected error: %v, but returned no error", test.Control, test.ExpectedError)
			}

			if test.ExpectedResponse != nil && res == nil {
				t.Error("Run with control: %v should have returned a result, but did not", test.Control)
			}

		}

		controller.StopClient()

	}
}
