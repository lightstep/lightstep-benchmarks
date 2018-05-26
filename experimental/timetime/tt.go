// Compute the cost of calling time.Now()
package main

import (
	"time"

	"github.com/lightstep/lightstep-benchmarks/experimental/diffbench"
)

func main() {
	test := func(x int32) int32 {
		return x ^ int32(time.Now().UnixNano())
	}

	diffbench.RunAndSave("System clock timing", test)
}
