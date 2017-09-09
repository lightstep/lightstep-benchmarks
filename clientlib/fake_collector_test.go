package clientlib

import (
	"github.com/lightstep/lightstep-benchmarks/benchlib"
	"testing"
)

var (
	GrpcPort = 8001
	HTTPPort = 8002
)

type FakeCollectorTest struct {
	Control               benchlib.Control
	Client                TestClient
	GrpcPort              *int
	HTTPPort              *int
	ExpectedSpansReceived int64
	ExpectedSpansDropped  int64
	ExpectedBytesReceived int64
}

var fakeCollectorTestRuns = []FakeCollectorTest{
	FakeCollectorTest{
		Control: benchlib.Control{
			Concurrent:    2,
			Work:          1000,
			Repeat:        10,
			Trace:         true,
			Sleep:         1,
			SleepInterval: 5,
		},
		Client:                Clients["golang"],
		GrpcPort:              &GrpcPort,
		HTTPPort:              nil,
		ExpectedSpansDropped:  0,
		ExpectedBytesReceived: 0,
	},
	FakeCollectorTest{
		Control: benchlib.Control{
			Concurrent:    2,
			Work:          1000,
			Repeat:        100,
			Trace:         true,
			Sleep:         1,
			SleepInterval: 5,
		},
		Client:                Clients["golang"],
		GrpcPort:              &GrpcPort,
		HTTPPort:              nil,
		ExpectedSpansDropped:  0,
		ExpectedBytesReceived: 0,
	},
	FakeCollectorTest{
		Control: benchlib.Control{
			Concurrent:    2,
			Work:          1000,
			Repeat:        100,
			Trace:         false,
			Sleep:         1,
			SleepInterval: 5,
		},
		Client:                Clients["golang"],
		GrpcPort:              &GrpcPort,
		HTTPPort:              nil,
		ExpectedSpansDropped:  0,
		ExpectedBytesReceived: 0,
	},
	FakeCollectorTest{ // Should drop ~39000
		Control: benchlib.Control{
			Concurrent:    4,
			Work:          1,
			Repeat:        10000,
			Trace:         true,
			Sleep:         1,
			SleepInterval: 5,
		},
		Client:                Clients["golang"],
		GrpcPort:              &GrpcPort,
		HTTPPort:              nil,
		ExpectedSpansDropped:  39000,
		ExpectedBytesReceived: 0,
	},
}

func TestFakeCollectorGRPC(t *testing.T) {
	clientController := CreateHTTPTestClientController()
	clientController.StartControlServer()

	fc := CreateFakeCollector()

	for _, test := range fakeCollectorTestRuns {
		fc.Run(test.HTTPPort, test.GrpcPort)
		_ = clientController.StartClient(test.Client)

		_, _ = clientController.Run(test.Control)

		clientController.StopClient()

		spansReceived, spansDropped, bytesReceived := fc.GetStats()

		var expectedSpansReceived int64
		if test.Control.Trace {
			expectedSpansReceived = int64(test.Control.Concurrent)*test.Control.Repeat - test.ExpectedSpansDropped
		} else {
			expectedSpansReceived = 0
		}

		if spansReceived != expectedSpansReceived {
			t.Errorf("Fake collector should have recieved %v spans, but found %v instead", expectedSpansReceived, spansReceived)
		}

		if spansDropped != test.ExpectedSpansDropped {
			t.Errorf("Fake collector should have dropped %v spans, but found %v instead", test.ExpectedSpansDropped, spansDropped)
		}

		if bytesReceived != test.ExpectedBytesReceived {
			t.Errorf("Fake collector should have received %v bytes, but found %v instead", test.ExpectedBytesReceived, bytesReceived)
		}
		fc.ResetStats()

		spansReceived, spansDropped, bytesReceived = fc.GetStats()
		if spansReceived != 0 || spansDropped != 0 || bytesReceived != 0 {
			t.Errorf("Fake collector stats not reset, found (%v, %v, %v)", spansReceived, spansDropped, bytesReceived)
		}

		fc.Stop()
	}

}
