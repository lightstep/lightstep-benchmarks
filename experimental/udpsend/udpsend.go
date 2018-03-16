// A differential microbenchmark to compute the cost of sending a
// small packet interleaved with a CPU-bound process.
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
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
	numTrials = 100000

	// sendSize is the number of bytes in the UDP packet.
	sendSize = 200

	// The control factor indicates how much longer the synthetic
	// control computation takes, compared to the rough estimate
	// of the experiment.  The maximum value is so that a sparse
	// array can be used.
	maxControlFactor = 10000

	// The repeat parameter multiplies the number of times both
	// the work/send computation is repeated in a single trial
	// measurement.
	maxRepetitionFactor = 10
)

var (
	// Tested values for the work
	experimentParams = intRange(100, 1000, 50)

	// Tested values for the repeat parameter.
	repetitionParams = []int{2, 4, 6, 8, 10}

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

	// testTrials collects the array of [experimentParams x
	// repetitionParams] measurements.
	testTrials       [maxRepetitionFactor + 1]testMeasurements
	testMeasurements [maxControlFactor + 1]Timings
)

// The exported types
type (
	Measurements struct {
		// Indexed by the backoff factor
		Backoff map[int]*Timings
	}

	Trials struct {
		// Indexed by the repetition factor
		Repeat map[int]Measurements
	}

	Exported struct {
		RepeatParams     []int
		ExperimentParams []int
		Control          Trials
		Experiment       Trials
	}

	Timings []common.Timing
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
	for _, w := range experimentParams {
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
// the logic for appending new trial results in `measure`, and avoids
// memory growth during the test.
func emptyResults() *testResults {
	results := &testResults{}
	for on := 0; on < 2; on++ {
		for _, r := range repetitionParams {
			for _, w := range experimentParams {
				results[on][r][w] = make([]common.Timing, 0, numTrials)
			}
		}
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
		results[tp.featureOn][tp.repetition][tp.control] =
			append(results[tp.featureOn][tp.repetition][tp.control], diff)
	}
	return results
}

// computeRoughEstimate returns `roughEstimate` and `workFactor`.  The
// rough estimate is the estimated cost of a UDP send, taken using
// `testing.Benchmark`.  The busywork function `somework(workFactor)`
// has duration approximately equal to `roughEstimate`.
func computeRoughEstimate(test func(int32)) (roughEstimate common.Time, workFactor int) {
	fmt.Println("# work params", experimentParams)
	fmt.Println("# repeat params", repetitionParams)
	var rough common.Stats
	var work common.Stats
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

func writeTo(name string, data []byte) {
	if err := ioutil.WriteFile(name, data, os.ModePerm); err != nil {
		panic(err)
	}
}

func save(results *testResults) {
	exp := &Exported{
		RepeatParams:     repetitionParams,
		ExperimentParams: experimentParams,
		Control:          toTrials(&results[0]),
		Experiment:       toTrials(&results[1]),
	}
	data, err := json.MarshalIndent(exp, "", "  ")
	if err != nil {
		panic(err)
	}
	writeTo("output", data)
}

func toTrials(tt *testTrials) Trials {
	t := Trials{
		Repeat: map[int]Measurements{},
	}
	for _, r := range repetitionParams {
		t.Repeat[r] = toMeasurements(&tt[r])
	}
	return t
}

func toMeasurements(tm *testMeasurements) Measurements {
	m := Measurements{
		Backoff: map[int]*Timings{},
	}
	for _, e := range experimentParams {
		m.Backoff[e] = &tm[e]
	}
	return m
}

func main() {
	conn := connectUDP()
	test := func(id int32) { udpSend(id, conn) }
	results := measure(test)
	save(results)
}
