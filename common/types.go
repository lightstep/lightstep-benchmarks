package common

import "time"

type (
	Params struct {
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

		SysInterferenceThreshold  float64
		UserInterferenceThreshold float64
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

	Result struct {
		// The client under test measures its walltime, the controller
		// measures user and system time. These are the raw values.
		Measured Timing

		Flush Timing

		// Sleeps is the sum of about the sleep operations observed
		// by the client, in seconds of walltime.
		Sleeps Time
	}

	Config struct {
		Concurrency int
		LogNum      int64
		LogSize     int64
	}

	DataPoint struct {
		RequestRate float64 // Number of operations per second
		WorkRatio   float64 // Measured work rate
		SleepRatio  float64 // Measured sleep rate
	}

	Measurement struct {
		TargetRate float64
		TargetLoad float64
		Untraced   DataPoint // Tracing off
		Traced     DataPoint // Tracing on
		Completion float64   // Tracing on completion rate
	}

	// Finished results format.
	Output struct {
		// Settings
		Title      string // Test title
		Client     string // Test client name
		Name       string // Test config name
		Concurrent int    // Number of concurrent threads
		LogBytes   int64  // Number of bytes of log per span

		// Experiment data
		Results []Measurement
	}
)
