// Compute the cost of calling time.Now()
package main

import (
	"time"

	"github.com/lightstep/lightstep-benchmarks/experimental/diffbench"
)

func main() {
	test := func(id int32) { time.Now() }

	diffbench.RunAndSave("output", test)
}
