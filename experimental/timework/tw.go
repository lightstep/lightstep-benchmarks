package main

import "github.com/lightstep/lightstep-benchmarks/experimental/diffbench"

func main() {
	diffbench.RunAndSave("output", func(x int32) int32 {
		return diffbench.WorkFunc(x, 1000)
	})
}
