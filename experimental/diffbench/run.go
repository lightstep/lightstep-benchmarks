// A differential microbenchmark to compute the cost of a function
// interleaved within a CPU-bound process.
package diffbench

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
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
	// busywork function and the cost of the operation.  We take
	// the mean from this many tests for the initial estimates.
	roughTrials = 10

	// The expreriment performs numTrials for each combination of
	// 'work' multiplier and 'repeat' parameter.
	numTrials = 250000

	// This many repeats are used to rough-calibrate the work function.
	workFunctionCalibrationFactor = 1e8
)

var (
	// Tested values for the work
	experimentParams = intRange(10, 100, 10)

	// Tested values for the repeat parameter.
	repetitionParams = []int{1, 2, 3, 4, 5}
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

	Measurements struct {
		// Indexed by the backoff factor
		Backoff map[int]*Timings
	}

	Trials struct {
		// Indexed by the repetition factor
		Repeat map[int]*Measurements
	}

	Exported struct {
		RepeatParams     []int
		ExperimentParams []int
		Control          *Trials
		Experiment       *Trials
	}

	Timings []common.Timing
)

// intRange returns a slice of ints from low to high in steps.
func intRange(low, high, step int) []int {
	var r []int
	for i := low; i <= high; i += step {
		r = append(r, i)
	}
	return r
}

// WorkFunc is the busywork function
func WorkFunc(in int32, count int) int32 {
	for ; count != 0; count-- {
		in *= 982451653
	}
	return in
}

// experimentTestParams returns a full experiment worth of test
// parameters, randomly shuffled.
func experimentTestParams() (params []testParams) {
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
func emptyResults() *Exported {
	newMeas := func() *Measurements {
		m := &Measurements{Backoff: map[int]*Timings{}}
		for _, b := range experimentParams {
			timings := make(Timings, 0, numTrials)
			m.Backoff[b] = &timings
		}
		return m
	}
	newTrials := func() *Trials {
		t := &Trials{Repeat: map[int]*Measurements{}}
		for _, r := range repetitionParams {
			t.Repeat[r] = newMeas()
		}
		return t
	}
	return &Exported{
		RepeatParams:     repetitionParams,
		ExperimentParams: experimentParams,
		Control:          newTrials(),
		Experiment:       newTrials(),
	}
}

func (e *Exported) trialsFor(on int) *Trials {
	if on != 0 {
		return e.Experiment
	}
	return e.Control
}

// measure performs the complete experiment and returns the results.
func measure(test func(int32) int32) *Exported {
	roughEstimate, workFactor := computeRoughEstimate(test)
	params := experimentTestParams()
	results := emptyResults()
	approx := common.Time(0)
	for _, tp := range params {
		approx += roughEstimate * common.Time((2*tp.control+tp.featureOn)*tp.repetition)
	}
	fmt.Println("# experiments will take approximately", approx, "at", time.Now())
	for _, tp := range params {
		runtime.GC()
		before := bench.GetSelfUsage()
		value := int32(1)
		for iter := 0; iter < tp.repetition; iter++ {
			value = WorkFunc(value, tp.control*workFactor)
			if tp.featureOn != 0 {
				value = test(value)
			}
		}
		after := bench.GetSelfUsage()
		diff := after.Sub(before).Div(float64(tp.repetition))
		timings := results.trialsFor(tp.featureOn).Repeat[tp.repetition].Backoff[tp.control]
		(*timings) = append(*timings, diff)
	}
	return results
}

// computeRoughEstimate returns `roughEstimate` and `workFactor`.  The
// rough estimate is the estimated cost of a UDP send, taken using
// `testing.Benchmark`.  The busywork function `somework(workFactor)`
// has duration approximately equal to `roughEstimate`.
func computeRoughEstimate(test func(int32) int32) (roughEstimate common.Time, workFactor int) {
	fmt.Println("# work params", experimentParams)
	fmt.Println("# repeat params", repetitionParams)
	var rough common.Stats
	var work common.Stats

	for i := 0; i < roughTrials; i++ {
		rough1 := testing.Benchmark(func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				test(1<<31 - 1)
			}
		})
		rough.Update(rough1.T.Seconds() / float64(rough1.N))

		work1 := testing.Benchmark(func(b *testing.B) {
			_ = WorkFunc(1, b.N*workFunctionCalibrationFactor)
		})
		work.Update(work1.T.Seconds() / float64(work1.N) / workFunctionCalibrationFactor)
	}
	roughEstimate = common.Time(rough.Mean())
	roughWork := work.Mean()
	workFactor = int(roughEstimate.Seconds() / roughWork)
	fmt.Printf("# rough estimate %v\n# work timing %v\n", roughEstimate, roughWork)
	return
}

func writeTo(name string, data []byte) {
	if err := ioutil.WriteFile(name, data, os.ModePerm); err != nil {
		panic(err)
	}
}

func save(file string, exp *Exported) {
	data, err := json.Marshal(exp)
	if err != nil {
		panic(err)
	}
	writeTo(file, data)
}

func RunAndSave(file string, test func(id int32) int32) {
	save(file, measure(test))
}
