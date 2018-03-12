package common

import (
	"fmt"

	"gonum.org/v1/gonum/stat"
)

type (
	Stats []float64

	TimingStats struct {
		Wall, User, Sys Stats
	}
)

func NewTimingStats(ts []Timing) TimingStats {
	s := TimingStats{}
	for _, t := range ts {
		s.Update(t)
	}
	return s
}

func (s *Stats) Update(v float64) {
	*s = append(*s, v)
}

func (s *Stats) Count() int {
	return len(*s)
}

func (s *Stats) NormalConfidenceInterval() (low, high float64) {
	// For a 95% confidence interval
	const ninetyFiveConfidenceZValue = 1.96

	m, std := stat.MeanStdDev(*s, nil)
	se := stat.StdErr(std, float64(s.Count()))
	return (m + ninetyFiveConfidenceZValue*se), (m - ninetyFiveConfidenceZValue*se)
}

func (s *Stats) Mean() float64 {
	return stat.Mean(*s, nil)
}

func (ts *TimingStats) Update(tm Timing) {
	ts.Wall.Update(tm.Wall.Seconds())
	ts.User.Update(tm.User.Seconds())
	ts.Sys.Update(tm.Sys.Seconds())
}

func (ts *TimingStats) Mean() Timing {
	return Timing{
		Time(ts.Wall.Mean()),
		Time(ts.User.Mean()),
		Time(ts.Sys.Mean()),
	}
}

func (ts *TimingStats) NormalConfidenceInterval() (low, high Timing) {
	wl, wh := ts.Wall.NormalConfidenceInterval()
	ul, uh := ts.User.NormalConfidenceInterval()
	sl, sh := ts.Sys.NormalConfidenceInterval()
	return Timing{Time(wl), Time(ul), Time(sl)}, Timing{Time(wh), Time(uh), Time(sh)}
}

func (ts *TimingStats) Count() int {
	return ts.Wall.Count()
}

func (ts TimingStats) String() string {
	l, h := ts.NormalConfidenceInterval()
	return fmt.Sprintf("[%v - %v]", l, h)
}
