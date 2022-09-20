package main

import (
	"flag"
	"math"
)

var argTracer = flag.String("traceR", "", "Which Lightstep tracer to use")
var argTrace = flag.Int("trace", 0, "Whether to trace")
var argSleep = flag.Float64("sleep", 0, "The amount of time to sleep for each span")
var argSleepInterval = flag.Int("sleep_interval", 0, "The duration of each sleep")
var argWork = flag.Int("work", 0, "The quanitity of work to perform between spans")
var argRepeat = flag.Int("repeat", 0, "The number of span generation repetitions to perform")
var argNoFlush = flag.Int("no_flush", 0, "Whether to flush on finishing")
var argNumTags = flag.Int("num_tags", 0, "The number of tags to set on a span")
var argNumLogs = flag.Int("num_logs", 0, "The number of logs to set on a span")

var workResult = 0.0
var tagKeys []string = nil
var tagVals []string = nil
var logKeys []string = nil
var logVals []string = nil

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func doWork(units int) {
	// Follows the approach outlined in
	// https://stackoverflow.com/a/36975497/4447365
	// to prevent the compiler from optimizing out the result
	x := 1.12563
	for i := 0; i < units; i++ {
		x *= math.Sqrt(math.Log(float64(i + 5)))
	}
	workResult = x
}

func setupAnnotations() {

}

func performWork() {

}

func main() {
	flag.Parse()
	setupAnnotations()
	performWork()
}
