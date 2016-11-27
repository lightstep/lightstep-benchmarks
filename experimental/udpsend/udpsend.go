package main

import (
	"fmt"
	"math"
	"math/rand"
	"net"
	"runtime"
	"strings"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/lightstep/lightstep-benchmarks/benchlib"
)

const (
	minRepeatParam = 100
	maxRepeatParam = 1000
	repeatStep     = 100

	minWorkParam = 10
	maxWorkParam = 150
	workStep     = 10

	numTrials = 500

	numKeys = 10
	keySize = 10
	valSize = 10
)

var (
	roughEstimate benchlib.Time // estimate by testing.Benchmark()
	workFactor    int           // someWork(workFactor) takes ~roughEstimate
)

type tParam struct {
	parameter  int
	iterations int
	featureOn  int // 0 or 1
}

// Note! This is array may be sparsely used.
type tExperiment [maxWorkParam + 1][maxRepeatParam + 1][]benchlib.Timing
type tResults [2]tExperiment

func someWork(c int) int32 {
	s := int32(1)
	for ; c != 0; c-- {
		s *= 982451653
	}
	return s
}

func udpSend(id int32, conn *net.UDPConn) {
	r := &Report{}
	r.Id = id
	r.Field = make([]*KeyValue, numKeys)
	for i, _ := range r.Field {
		r.Field[i] = &KeyValue{strings.Repeat("k", keySize), strings.Repeat("v", valSize)}
	}
	d, err := proto.Marshal(r)
	if err != nil {
		panic(err.Error())
	}
	conn.Write(d)
}

func connectUDP() *net.UDPConn {
	udpAddr, err := net.ResolveUDPAddr("udp", "localhost:1025")
	if err != nil {
		panic(err.Error())
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		panic(err.Error())
	}
	return conn
}

func getParams() []tParam {
	var params []tParam
	for w := minWorkParam; w <= maxWorkParam; w += workStep {
		for r := minRepeatParam; r <= maxRepeatParam; r += repeatStep {
			for t := 0; t < numTrials; t++ {
				params = append(params, tParam{w, r, 1})
				params = append(params, tParam{w, r, 0})
			}
		}
	}
	for i := 1; i < len(params); i++ {
		ri := rand.Intn(i)
		params[ri], params[i] = params[i], params[ri]
	}
	return params
}

func emptyResults() *tResults {
	results := &tResults{}
	for on := 0; on < 2; on++ {
		exp := tExperiment{}
		for w := minWorkParam; w <= maxWorkParam; w += workStep {
			for r := minRepeatParam; r <= maxRepeatParam; r += repeatStep {
				exp[w][r] = make([]benchlib.Timing, 0, numTrials)
			}
		}
		results[on] = exp
	}
	return results
}

func measure(test func(int32)) *tResults {
	params := getParams()
	results := emptyResults()
	approx := benchlib.Time(0)
	for _, tp := range params {
		approx += roughEstimate * benchlib.Time((tp.parameter+tp.featureOn)*tp.iterations)
	}
	fmt.Println("experiments should take approximately", approx)
	for _, tp := range params {
		runtime.GC()
		before := benchlib.GetSelfUsage()
		for iter := 0; iter < maxRepeatParam; iter++ {
			value := someWork(tp.parameter * workFactor)
			if tp.featureOn != 0 {
				test(value)
			}
		}
		after := benchlib.GetSelfUsage()
		diff := after.Sub(before).Div(maxRepeatParam)
		results[tp.featureOn][tp.parameter][tp.iterations] =
			append(results[tp.featureOn][tp.parameter][tp.iterations], diff)
	}
	return results
}

func computeConstants(test func(int32)) {
	rough := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			test(math.MaxInt32)
		}
	})
	work1M := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			someWork(1e6)
		}
	})
	roughEstimate = benchlib.Time(rough.T.Seconds() / float64(rough.N))
	roughWork := work1M.T.Seconds() / float64(work1M.N) / 1e6
	workFactor = int(roughEstimate.Seconds() / roughWork)
}

func show(results *tResults) {
	for w := minWorkParam; w <= maxWorkParam; w += workStep {
		for r := minRepeatParam; r <= maxRepeatParam; r += repeatStep {

			off := benchlib.NewTimingStats(results[0][w][r])
			on := benchlib.NewTimingStats(results[1][w][r])

			onlow, _ := on.NormalConfidenceInterval()
			_, offhigh := off.NormalConfidenceInterval()
			fmt.Printf("W/R=%v/%v  MDIFF=%v SPREAD=%v\n",
				w, r, on.Mean().Sub(off.Mean()), onlow.Sub(offhigh))
		}
	}
}

func main() {
	conn := connectUDP()
	test := func(id int32) { udpSend(id, conn) }

	computeConstants(test)
	results := measure(test)

	show(results)
}
