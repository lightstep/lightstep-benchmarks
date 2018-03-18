package common

import (
	"sort"

	"gonum.org/v1/gonum/stat"
)

type (
	Stats []float64

	TimingStats struct {
		Wall, User, Sys Stats
	}

	StatsSummary struct {
		ZValue
		CLow  float64
		CHigh float64
		Mean  float64
		Min   float64
		P25   float64
		P50   float64
		P75   float64
		Max   float64
	}

	StatsSeries struct {
		Coordinates []float64
		Summaries   []StatsSummary
	}

	ZValue struct {
		C float64
		Z float64
	}
)

var (
	C70 = ZValue{C: 70, Z: 1.04}
	C75 = ZValue{C: 75, Z: 1.15}
	C80 = ZValue{C: 80, Z: 1.28}
	C85 = ZValue{C: 85, Z: 1.44}
	C90 = ZValue{C: 90, Z: 1.645}
	C92 = ZValue{C: 92, Z: 1.75}
	C95 = ZValue{C: 95, Z: 1.96}
	C96 = ZValue{C: 96, Z: 2.05}
	C98 = ZValue{C: 98, Z: 2.33}
	C99 = ZValue{C: 99, Z: 2.58}

	ConfidenceAll = []ZValue{C70, C75, C80, C85, C90, C92, C95, C96, C98, C99}
)

func NewTimingStats(ts []Timing) *TimingStats {
	s := &TimingStats{}
	for _, t := range ts {
		s.Update(t)
	}
	return s
}

func (s *Stats) Update(v float64) {
	*s = append(*s, v)
}

func (s Stats) Count() int {
	return len(s)
}

func (s Stats) Summary(z ZValue) StatsSummary {
	sort.Float64s(s)

	m, std := stat.MeanStdDev(s, nil)
	se := stat.StdErr(std, float64(s.Count()))

	return StatsSummary{
		ZValue: z,
		CLow:   (m - z.Z*se),
		CHigh:  (m + z.Z*se),
		Mean:   m,
		Min:    s[0],
		Max:    s[len(s)-1],
		P25:    s[len(s)/4],
		P50:    s[len(s)/2],
		P75:    s[3*len(s)/4],
	}
}

func (s Stats) Mean() float64 {
	return stat.Mean(s, nil)
}

func (a Stats) Combine(b Stats) (c Stats) {
	for i := range a {
		c = append(c, a[i]+b[i])
	}
	return
}

func (ss StatsSummary) SubScalar(x float64) StatsSummary {
	return StatsSummary{
		ZValue: ss.ZValue, // TODO Not really @@@
		CLow:   ss.CLow - x,
		CHigh:  ss.CHigh - x,
		Mean:   ss.Mean - x,
		Min:    ss.Min - x,
		P25:    ss.P25 - x,
		P50:    ss.P50 - x,
		P75:    ss.P75 - x,
		Max:    ss.Max - x,
	}
}

func (ss *StatsSeries) Add(coord float64, s StatsSummary) {
	ss.Coordinates = append(ss.Coordinates, coord)
	ss.Summaries = append(ss.Summaries, s)
}

func (ts *TimingStats) Update(tm Timing) {
	ts.Wall.Update(tm.Wall.Seconds())
	ts.User.Update(tm.User.Seconds())
	ts.Sys.Update(tm.Sys.Seconds())
}

// func (ts *TimingStats) Count() int {
// 	return ts.Wall.Count()
// }

// func (ts *TimingStats) String() string {
// 	l, h := ts.NormalConfidenceInterval()
// 	return fmt.Sprintf("[%v - %v]", l, h)
// }

func (ts *TimingStats) UserSys() Stats {
	return ts.User.Combine(ts.Sys)
}
