package common

import (
	"fmt"
	"strconv"
	"syscall"
	"time"

	"github.com/lightstep/lightstep-benchmarks/env"
)

type (
	Time float64

	Timing struct {
		Wall, User, Sys Time
	}
)

func ParseTime(s string) Time {
	timing, err := strconv.ParseFloat(s, 64)
	if err != nil {
		env.Fatal("Could not parse timing: ", s, ": ", err)
	}
	return Time(timing)
}

func WallTiming(seconds float64) Timing {
	return Timing{Wall: Time(seconds)}
}

func (t Time) Seconds() float64 {
	return float64(t)
}

func (t Time) Duration() Duration {
	return Duration(t * Time(time.Second))
}

func (t Time) Nanoseconds() int64 {
	return int64(float64(t) * 1e9)
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

func Timeval(t syscall.Timeval) Time {
	return Time(float64(t.Sec) + float64(t.Usec)*1e-6)
}

func (t Time) String() string {
	if t < 10e-9 && t > -10e-9 {
		return fmt.Sprintf("%.3fns", float64(t)*1e9)
	}
	return t.Duration().String()
}

func (ts Timing) String() string {
	return fmt.Sprintf("W: %v U: %v S: %v", ts.Wall, ts.User, ts.Sys)
}

func (ts Timing) RawString() string {
	return fmt.Sprintf("%e %e %e", ts.Wall, ts.User, ts.Sys)
}
