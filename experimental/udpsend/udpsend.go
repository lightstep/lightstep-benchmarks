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
	numKeys = 10
	keySize = 10
	valSize = 10

	minParam = 1
	maxParam = 150

	numTrials = 300
)

var (
	roughEstimate benchlib.Time // estimate by testing.Benchmark()
	workFactor    int           // someWork(workFactor) takes ~roughEstimate
)

type tParam struct {
	parameter int
	featureOn int
}

type tResults [2][maxParam + 1][]benchlib.Timing

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
	for i := minParam; i <= maxParam; i++ {
		for j := 0; j < numTrials; j++ {
			params = append(params, tParam{i, 1})
			params = append(params, tParam{i, 0})
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
		pslices := [maxParam + 1][]benchlib.Timing{}
		for i := 0; i <= maxParam; i++ {
			pslices[i] = make([]benchlib.Timing, 0, numTrials)
		}
		results[on] = pslices
	}
	return results
}

func measure(test func(int32)) *tResults {
	params := getParams()
	results := emptyResults()

	for _, tp := range params {
		runtime.GC()
		before := benchlib.GetSelfUsage()

		value := someWork(tp.parameter * workFactor)
		if tp.featureOn != 0 {
			test(value)
		}

		after := benchlib.GetSelfUsage()
		diff := after.Sub(before)
		results[tp.featureOn][tp.parameter] = append(results[tp.featureOn][tp.parameter], diff)
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
	for p := minParam; p <= maxParam; p++ {
		off := benchlib.NewTimingStats(results[0][p])
		on := benchlib.NewTimingStats(results[1][p])

		onlow, _ := on.NormalConfidenceInterval()
		_, offhigh := off.NormalConfidenceInterval()
		//fmt.Printf("P=%v OFF=%v\n", p, off)
		//fmt.Printf("P=%v  ON=%v\n", p, on)
		fmt.Printf("P=%v SPREAD=%v\n", p, onlow.Sub(offhigh))
	}
}

func main() {
	conn := connectUDP()
	test := func(id int32) { udpSend(id, conn) }

	computeConstants(test)
	results := measure(test)

	show(results)
}
