// A differential microbenchmark to compute the cost of sending a
// small packet interleaved with a CPU-bound process.
package main

import (
	"fmt"
	"math/rand"
	"net"
	"runtime"
	"testing"
	"time"

	"github.com/lightstep/lightstep-benchmarks/bench"
	"github.com/lightstep/lightstep-benchmarks/common"
)

const (
	// Before running the experiment, `testing.Benchmark` is used
	// to calculate a rough estimate for both the cost of the
	// busywork function and the cost of the UDP packet.  We take
	// the mean from this many tests for the initial estimates.
	roughTrials = 10

	// The expreriment performs numTrials for each combination of
	// 'work' multiplier and 'repeat' parameter.
	numTrials = 5000

	// sendSize is the number of bytes in the UDP packet.
	sendSize = 200

	// The control factor indicates how much longer the synthetic
	// control computation takes, compared to the rough estimate
	// of the experiment.  The maximum value is so that a sparse
	// array can be used.
	maxControlFactor = 10000

	// The repeat parameter multiplies the number of times both
	// the work/send computation is repeated in a single trial
	// measurement.  We have a null hypothesis that this variable
	// has no effect.
	maxRepetitionFactor = 10
)

var (
	// Tested values for the work parameter.
	controlParams = append(intRange(100, 1900, 100), intRange(2000, 10000, 1000)...)

	// Tested values for the repeat parameter.
	repetitionParams = []int{1, 2, 4, 8}

	// The blank array used for sending.
	sendBuffer = make([]byte, sendSize)
)

type (
	// testParams describes a single trial measurement.
	testParams struct {
		// control is the ratio between the busywork duration
		// and the estimated experiment duration.
		control int
		// repetition is the number of times the busywork/send
		// is repeated.
		repetition int
		// is this testing the experiment or the control? 1 or 0
		featureOn int
	}

	// testResults is indexed by featureOn == 0 or 1
	testResults [2]testTrials

	// testTrials collects the array of [controlParams x
	// repetitionParams] measurements.
	testTrials [maxControlFactor + 1][maxRepetitionFactor + 1][]common.Timing
)

// init fills in `sendBuffer` with random bytes.
func init() {
	for i := range sendBuffer {
		sendBuffer[i] = byte(rand.Intn(256))
	}
}

// intRange returns a slice of ints from low to high in steps.
func intRange(low, high, step int) []int {
	var r []int
	for i := low; i <= high; i += step {
		r = append(r, i)
	}
	return r
}

// someWork is the busywork function
func someWork(c int) int32 {
	s := int32(1)
	for ; c != 0; c-- {
		s *= 982451653
	}
	return s
}

// udpSend is the function being measured.
func udpSend(id int32, conn *net.UDPConn) {
	// Prevent the compiler from observing the unused variable.
	sendBuffer[0] = byte(id & 0xff)
	if n, err := conn.Write(sendBuffer); err != nil || n != len(sendBuffer) {
		panic(err.Error())
	}
}

// connectUDP returns a connection for testing with.
func connectUDP() *net.UDPConn {
	// Note: /255 is a broadcast address, this prevents the
	// connection from failure (assumes netmask is /24).
	address := "192.168.0.255:8765"

	raddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		panic(err.Error())
	}

	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		panic(err.Error())
	}
	return conn
}

// experimentTestParams returns a full experiment worth of test
// parameters, randomly shuffled.
func experimentTestParams() []testParams {
	var params []testParams
	for _, w := range controlParams {
		for _, r := range repetitionParams {
			for t := 0; t < numTrials; t++ {
				params = append(params, testParams{w, r, 1})
				params = append(params, testParams{w, r, 0})
			}
		}
	}
	rand.Shuffle(len(params), func(i, j int) {
		params[j], params[i] = params[i], params[j]
	})
	return params
}

// emptyResults returns an empty table of results, which simplifies
// the logic for appending new trial results in `measure`.
func emptyResults() *testResults {
	results := &testResults{}
	for on := 0; on < 2; on++ {
		exp := testTrials{}
		for _, w := range controlParams {
			for _, r := range repetitionParams {
				exp[w][r] = make([]common.Timing, 0, numTrials)
			}
		}
		results[on] = exp
	}
	return results
}

// measure performs the complete experiment and returns the results.
func measure(test func(int32)) *testResults {
	roughEstimate, workFactor := computeRoughEstimate(test)
	params := experimentTestParams()
	results := emptyResults()
	approx := common.Time(0)
	for _, tp := range params {
		approx += roughEstimate * common.Time((tp.control+tp.featureOn)*tp.repetition)
	}
	fmt.Println("# experiments will take approximately", approx, "at", time.Now())
	for _, tp := range params {
		runtime.GC()
		before := bench.GetSelfUsage()
		for iter := 0; iter < tp.repetition; iter++ {
			value := someWork(tp.control * workFactor)
			if tp.featureOn != 0 {
				test(value)
			}
		}
		after := bench.GetSelfUsage()
		diff := after.Sub(before).Div(float64(tp.repetition))
		results[tp.featureOn][tp.control][tp.repetition] =
			append(results[tp.featureOn][tp.control][tp.repetition], diff)
	}
	return results
}

// computeRoughEstimate returns `roughEstimate` and `workFactor`.  The
// rough estimate is the estimated cost of a UDP send, taken using
// `testing.Benchmark`.  The busywork function `somework(workFactor)`
// has duration approximately equal to `roughEstimate`.
func computeRoughEstimate(test func(int32)) (roughEstimate common.Time, workFactor int) {
	fmt.Println("# work params", controlParams)
	fmt.Println("# repeat params", repetitionParams)
	var rough bench.Stats
	var work bench.Stats
	const large = 1e8 // this many repeats to rough calibrate work function

	for i := 0; i < roughTrials; i++ {
		rough1 := testing.Benchmark(func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				test(1<<31 - 1)
			}
		})
		rough.Update(rough1.T.Seconds() / float64(rough1.N))

		work1 := testing.Benchmark(func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				someWork(large)
			}
		})
		work.Update(work1.T.Seconds() / float64(work1.N) / large)
	}
	roughEstimate = common.Time(rough.Mean())
	roughWork := work.Mean()
	workFactor = int(roughEstimate.Seconds() / roughWork)
	fmt.Printf("# udp send rough estimate %v\n# work timing %v\n", roughEstimate, roughWork)
	return
}

func show(results *testResults) {
	for _, w := range controlParams {
		for _, r := range repetitionParams {
			off := common.NewTimingStats(results[0][w][r])
			on := common.NewTimingStats(results[1][w][r])

			onlow, _ := on.NormalConfidenceInterval()
			_, offhigh := off.NormalConfidenceInterval()
			fmt.Printf("# W/R=%v/%v MDIFF=%v SPREAD=%v\n",
				w, r, on.Mean().Sub(off.Mean()), onlow.Sub(offhigh))
		}
	}
	for _, r := range repetitionParams {
		fmt.Printf("# R=%v\n", r)
		for _, w := range controlParams {

			off := common.NewTimingStats(results[0][w][r])
			on := common.NewTimingStats(results[1][w][r])

			onlow, onhigh := on.NormalConfidenceInterval()
			offlow, offhigh := off.NormalConfidenceInterval()

			// Gross
			fmt.Printf("%v %v %.9f %.9f %.9f %.9f %.9f %.9f %.9f %.9f %.9f %.9f %.9f %.9f %.9f %.9f %.9f %.9f %.9f %.9f\n",
				w, r, // 1 2
				on.Wall.Mean(), onlow.Wall.Seconds(), onhigh.Wall.Seconds(), // 3 4/5
				off.Wall.Mean(), offlow.Wall.Seconds(), offhigh.Wall.Seconds(), // 6 7/8

				on.User.Mean(), onlow.User.Seconds(), onhigh.User.Seconds(), // 9 10/11
				off.User.Mean(), offlow.User.Seconds(), offhigh.User.Seconds(), // 12 13/14

				on.Sys.Mean(), onlow.Sys.Seconds(), onhigh.Sys.Seconds(), // 15 16/17
				off.Sys.Mean(), offlow.Sys.Seconds(), offhigh.Sys.Seconds()) // 18 19/20

			// Use gnuplot e.g., for walltime
			// plot 'data' using 1:($4-$6):($5-$6) with filledcurves lt rgb "gray" title '95% confidence',  '' using 1:($3-$6) with lines title 'mean value'

			// For user+system
			// plot 'n=10.dat' using 1:($10+$16-$12-$18):($11+$17-$12-$18) with filledcurves lt rgb "#b0b0b0" title '95% confidence', '' using 1:($9+$15-$12-$18) with lines title 'mean value', '' using 1:($13+$19-$12-$18):($14+$20-$12-$18) with filledcurves lt rgb "#d0d0d0" title '95% confidence', '' using 1:($12+$18-$12-$18) with lines title 'mean value'
			// For user
			// plot 'data.dat' using 1:($13-$12):($14-$12) with filledcurves lt rgb "#d0d0d0" title '95% confidence', '' using 1:($12-$12) with lines title 'mean value', '' using 1:($10-$12):($11-$12) with filledcurves lt rgb "#b0b0b0" title '95% confidence', '' using 1:($9-$12) with lines title 'mean value'

			// For system
			// plot 'data.dat' using 1:($19-$18):($20-$18) with filledcurves lt rgb "#d0d0d0" title '95% confidence', '' using 1:($18-$18) with lines title 'mean value', '' using 1:($16-$18):($17-$18) with filledcurves lt rgb "#b0b0b0" title '95% confidence', '' using 1:($15-$18) with lines title 'mean value'

		}
		fmt.Println("")
	}
}

func main() {
	conn := connectUDP()
	test := func(id int32) { udpSend(id, conn) }

	results := measure(test)

	show(results)
}
