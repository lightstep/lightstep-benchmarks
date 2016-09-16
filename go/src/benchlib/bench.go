package benchlib

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/GaryBoone/GoStats/stats"
)

const (
	ControllerPort        = 8000
	GrpcPort              = 8001
	ControllerHost        = "localhost"
	ControllerAccessToken = "ignored"

	ControlPath = "/control"
	ResultPath  = "/result"

	LogsSizeMax = 1 << 20

	nanosPerSecond = 1e9
)

var (
	// Tests amortize sleep calls so they're approximately this long.
	DefaultSleepInterval = 50 * time.Millisecond

	testVerbose = GetEnv("BENCHMARK_VERBOSE", "")
)

type Config struct {
	Concurrency int
	LogNum      int64
	LogSize     int64
}

type Control struct {
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
	Trace   bool // Trace the operation.
	Exit    bool // Terminate the test.
	Profile bool // Profile this operation
}

type Result struct {
	// The client under test measures its walltime, the controller
	// measures user and system time. These are the raw values.
	Measured Timing

	Flush Timing

	// The controller subtracts known overhead, yielding the
	// measurement attributed (according to the model) to the
	// Control (minus test / communication overhead).
	// Adjusted Timing

	// Sleeps are statistics about the sleep operations observed
	// by the client, in seconds of walltime.
	Sleeps Time
}

type DataPoint struct {
	RequestRate float64 // Number of operations per second
	WorkRatio   float64 // Measured work rate
	SleepRatio  float64 // Measured sleep rate
}

type Measurement struct {
	TargetRate float64
	TargetLoad float64
	Untraced   DataPoint // Tracing off
	Traced     DataPoint // Tracing on
	Completion float64   // Tracing on completion rate
}

// Finished results format.
type Output struct {
	// Settings
	Title      string // Test title
	Client     string // Test client name
	Name       string // Test config name
	Concurrent int    // Number of concurrent threads
	LogBytes   int64  // Number of bytes of log per span

	// Experiment data
	Results []Measurement
}

type Time float64

type Timing struct {
	Wall, User, Sys Time
}

type TimingStats struct {
	Wall, User, Sys stats.Stats
}

type Timings struct {
	X []float64
	Y []Timing
}

type Regression struct {
	Slope           Time
	Intercept       Time
	Rsquared        Time
	SlopeStdDev     Time
	InterceptStdDev Time
	Count           int
}

type TimingRegression struct {
	Wall Regression
	User Regression
	Sys  Regression
}

func GetEnv(name, defval string) string {
	if r := os.Getenv(name); r != "" {
		return r
	}
	return defval
}

func Fatal(x ...interface{}) {
	panic(fmt.Sprintln(x...))
}

func Print(x ...interface{}) {
	if testVerbose == "true" {
		fmt.Println(x...)
	}
}

func WallTiming(seconds float64) Timing {
	return Timing{Wall: Time(seconds)}
}

func linearRegression(x, y []float64) Regression {
	s, i, q, c, se, ie := stats.LinearRegression(x, y)
	return Regression{
		Count:           c,
		Slope:           Time(s),
		Intercept:       Time(i),
		Rsquared:        Time(q),
		SlopeStdDev:     Time(se),
		InterceptStdDev: Time(ie)}
}

func ParseTime(s string) Time {
	timing, err := strconv.ParseFloat(s, 64)
	if err != nil {
		Fatal("Could not parse timing: ", s, ": ", err)
	}
	return Time(timing)
}

func (ts *TimingStats) Update(tm Timing) {
	ts.Wall.Update(float64(tm.Wall))
	ts.User.Update(float64(tm.User))
	ts.Sys.Update(float64(tm.Sys))
}

func (ts *TimingStats) Mean() Timing {
	return Timing{
		Time(ts.Wall.Mean()),
		Time(ts.User.Mean()),
		Time(ts.Sys.Mean()),
	}
}

func (ts *TimingStats) StandardDeviation() Timing {
	return Timing{
		Time(ts.Wall.PopulationStandardDeviation()),
		Time(ts.User.PopulationStandardDeviation()),
		Time(ts.Sys.PopulationStandardDeviation()),
	}
}

func (ts *TimingStats) Count() int {
	return ts.Wall.Count()
}

func (t Time) Seconds() float64 {
	return float64(t)
}

func (t Time) Duration() time.Duration {
	return time.Duration(int64(t * nanosPerSecond))
}

func (t Timing) Sub(s Timing) Timing {
	t.Wall -= s.Wall
	t.User -= s.User
	t.Sys -= s.Sys
	return t
}

func (t Timing) Div(d float64) Timing {
	return Timing{t.Wall / Time(d), t.User / Time(d), t.Sys / Time(d)}
}

func (t Timing) SubFactor(s Timing, f float64) Timing {
	t.Wall -= s.Wall * Time(f)
	t.User -= s.User * Time(f)
	t.Sys -= s.Sys * Time(f)
	return t
}

func (d *Timings) Update(x float64, y Timing) {
	d.X = append(d.X, x)
	d.Y = append(d.Y, y)
}

func (d *Timings) LinearRegression() TimingRegression {
	x := d.X
	wally := make([]float64, len(x))
	usery := make([]float64, len(x))
	sysy := make([]float64, len(x))

	for i, y := range d.Y {
		wally[i] = y.Wall.Seconds()
		usery[i] = y.User.Seconds()
		sysy[i] = y.Sys.Seconds()
	}
	return TimingRegression{
		Wall: linearRegression(x, wally),
		User: linearRegression(x, usery),
		Sys:  linearRegression(x, sysy)}

}

func (d *TimingRegression) Slope() Timing {
	return Timing{
		Wall: d.Wall.Slope,
		User: d.User.Slope,
		Sys:  d.Sys.Slope,
	}
}

func (d *TimingRegression) Intercept() Timing {
	return Timing{
		Wall: d.Wall.Intercept,
		User: d.User.Intercept,
		Sys:  d.Sys.Intercept,
	}
}

func GetChildUsage(pid int) (Timing, CPUStat) {
	pstat := ProcessCPUStat(pid)
	// TODO hacky the 100.0 below is CLK_TCK (probably)
	return Timing{
		Wall: 0,
		User: Time(float64(pstat.User) / 100),
		Sys:  Time(float64(pstat.System) / 100),
	}, MachineCPUStat()
}

func (ts Timing) String() string {
	return fmt.Sprintf("W: %v U: %v S: %v", ts.Wall, ts.User, ts.Sys)
	// return ts.Wall.String()
}

func (ts TimingStats) String() string {
	return fmt.Sprintf("%v {%v[%v]}", ts.Mean(), ts.StandardDeviation(), ts.Count())
	//return fmt.Sprintf("%v", ts.Mean())
}

func (ts TimingRegression) String() string {
	// return fmt.Sprintf("W: %v U: %v S: %v", ts.Wall, ts.User, ts.Sys)
	return ts.Wall.String()
}

func (t Time) String() string {
	if t < 10e-9 && t > -10e-9 {
		return fmt.Sprintf("%.3fns", float64(t)*1e9)
	}
	return time.Duration(int64(t * nanosPerSecond)).String()
}

func (ts Regression) String() string {
	return fmt.Sprintf("[slope: %v @ %v]", ts.Slope, ts.Intercept)
}

func (dp DataPoint) VisibleImpairment() float64 {
	return 1 - dp.WorkRatio - dp.SleepRatio
}
