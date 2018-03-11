package bench

import (
	"syscall"
	"time"

	"github.com/lightstep/lightstep-benchmarks/common"
	"github.com/lightstep/lightstep-benchmarks/env"
)

const (
	ControllerPort        = 8000
	GrpcPort              = 8001
	ControllerHost        = "localhost"
	ControllerAccessToken = "ignored"

	ControlPath = "/control"
	ResultPath  = "/result"

	LogsSizeMax = 1 << 20
)

var (
	// Tests amortize sleep calls so they're approximately this long.
	DefaultSleepInterval = 50 * time.Millisecond
)

func GetChildUsage(pid int) (common.Timing, common.Timing, CPUStat) {
	selfTime := GetSelfUsage()
	pstat, err := ProcessCPUStat(pid)
	var childTime common.Timing
	if err != nil {
		// Note: this is to support development on machines w/o the proper /proc
		// files (e.g., on OS X).
		now := time.Now()
		secs := common.Time(float64(now.Unix()) + float64(now.UnixNano()/1e9))
		childTime = common.Timing{
			Wall: secs,
			User: secs,
			Sys:  secs,
		}
	} else {
		childTime = common.Timing{
			// TODO hacky the 100s below are CLK_TCK (sysconf(_SC_CLK_TCK) probably)
			Wall: 0,
			User: common.Time(float64(pstat.User) / 100),
			Sys:  common.Time(float64(pstat.System) / 100),
		}
	}
	return childTime, selfTime, MachineCPUStat()
}

func GetSelfUsage() common.Timing {
	var self syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &self); err != nil {
		env.Fatal("Can't getrusage(self)", err)
	}
	return common.Timing{
		Wall: common.Time(float64(time.Now().UnixNano()) / 1e9),
		User: common.Timeval(self.Utime),
		Sys:  common.Timeval(self.Stime)}
}
