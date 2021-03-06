package main

import (
	"context"
	"flag"
  "fmt"
	"github.com/lightstep/lightstep-tracer-go"
	"github.com/opentracing/opentracing-go"
	otlog "github.com/opentracing/opentracing-go/log"
	"math"
	"time"
)

const (
	satellitePort    = 8360
	reportingPeriod  = 200 * time.Millisecond
	maxBufferedSpans = 10000
	spansPerLoop     = 6
)

var argTracer = flag.String("tracer", "", "Which LightStep tracer to use")
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

func setupAnnotations() {
  tagKeys = make([]string, *argNumTags)
  tagVals = make([]string, *argNumTags)
  logKeys = make([]string, *argNumLogs)
  logVals = make([]string, *argNumLogs)
  for i := 0; i<*argNumTags; i++ {
    tagKeys[i] = fmt.Sprintf("tag.key%d", i)
    tagVals[i] = fmt.Sprintf("tag.value%d", i)
  }
  for i := 0; i<*argNumLogs; i++ {
    logKeys[i] = fmt.Sprintf("log.key%d", i)
    logVals[i] = fmt.Sprintf("log.value%d", i)
  }
}

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

func buildTracer() opentracing.Tracer {
	if *argTrace == 0 {
		return opentracing.NoopTracer{}
	}
	return lightstep.NewTracer(lightstep.Options{
		AccessToken: "developer",
		UseHttp:     true,
		Tags: map[string]interface{}{
			lightstep.ComponentNameKey: "isaac_service",
		},
		Collector: lightstep.Endpoint{
			Host:      "127.0.0.1",
			Port:      satellitePort,
			Plaintext: true,
		},
		ReportingPeriod:  reportingPeriod,
		MaxBufferedSpans: maxBufferedSpans,
	})
}

func makeSpan(tracer opentracing.Tracer, parent opentracing.SpanContext) opentracing.Span {
  span := tracer.StartSpan("isaac_service", opentracing.ChildOf(parent))
  for i := 0; i<*argNumTags; i++ {
    span.SetTag(tagKeys[i], tagVals[i])
  }
  for i := 0; i<*argNumLogs; i++ {
    span.LogFields(otlog.String(logKeys[i], logVals[i]))
  }
  return span
}


func generateSpans(tracer opentracing.Tracer, unitsWork int, numSpans int, parent opentracing.SpanContext) {
  client_span := makeSpan(tracer, parent)
	defer client_span.Finish()
	doWork(unitsWork)
	numSpans -= 1
	if numSpans == 0 {
		return
	}

  server_span := makeSpan(tracer, client_span.Context())
	defer server_span.Finish()
	doWork(unitsWork)
	numSpans -= 1
	if numSpans == 0 {
		return
	}

  db_span := makeSpan(tracer, server_span.Context())
	defer db_span.Finish()
	doWork(unitsWork)
	numSpans -= 1
	if numSpans == 0 {
		return
	}

	generateSpans(tracer, unitsWork, numSpans, server_span.Context())
}

func performWork() {
	tracer := buildTracer()

	sleepDebt := 0.0
	spansSent := 0
	for spansSent < *argRepeat {
		spansToSend := min(*argRepeat-spansSent, spansPerLoop)
		generateSpans(tracer, *argWork, spansToSend, nil)
		spansSent += spansToSend
		sleepDebt += *argSleep * float64(spansToSend)
		if sleepDebt > float64(*argSleepInterval) {
			sleepDebt -= float64(*argSleepInterval)
			time.Sleep(time.Duration(*argSleepInterval) * time.Nanosecond)
		}
	}
	if *argTrace != 0 && *argNoFlush != 1 {
		tracer.(lightstep.Tracer).Close(context.Background())
	}
}

func main() {
	flag.Parse()
  setupAnnotations()
	performWork()
}
