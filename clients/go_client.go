package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"time"

	"github.com/lightstep/lightstep-tracer-go"
	"github.com/opentracing/opentracing-go"
	otlog "github.com/opentracing/opentracing-go/log"
)

const (
	satellitePort = 8360
	spansPerLoop  = 6

	// These values match the default tracer configuration
	reportingPeriod    = 2500 * time.Millisecond
	minReportingPeriod = 100 * time.Millisecond
	maxBufferedSpans   = 1000
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

func setupAnnotations() {
	tagKeys = make([]string, *argNumTags)
	tagVals = make([]string, *argNumTags)
	logKeys = make([]string, *argNumLogs)
	logVals = make([]string, *argNumLogs)
	for i := 0; i < *argNumTags; i++ {
		tagKeys[i] = fmt.Sprintf("tag.key%d", i)
		tagVals[i] = fmt.Sprintf("tag.value%d", i)
	}
	for i := 0; i < *argNumLogs; i++ {
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
		// Set this to your access token and switch the Collector and SystemMetrics
		// entries below to report to Lightstep SaaS
		AccessToken: "developer",
		UseHttp:     true,
		Tags: map[string]interface{}{
			lightstep.ComponentNameKey: "go_benchmark_service",
		},
		// Comment this entry and uncomment the next one to report to Lightstep SaaS
		Collector: lightstep.Endpoint{
			Host:      "localhost",
			Port:      satellitePort,
			Plaintext: true,
		},
		//Collector: lightstep.Endpoint{
		//	Host:      "ingest.lightstep.com",
		//	Port:      443,
		//	Plaintext: false,
		//},
		ReportingPeriod:    reportingPeriod,
		MinReportingPeriod: minReportingPeriod,
		MaxBufferedSpans:   maxBufferedSpans,
		// Comment this entry and uncomment the next one to report to Lightstep SaaS
		SystemMetrics: lightstep.SystemMetricsOptions{
			Endpoint: lightstep.Endpoint{
				Host:      "localhost",
				Port:      8360,
				Plaintext: true,
			},
		},
		//SystemMetrics: lightstep.SystemMetricsOptions{
		//	Endpoint: lightstep.Endpoint{
		//		Host:      "ingest.lightstep.com",
		//		Port:      443,
		//		Plaintext: false,
		//	},
		//},
		UseGRPC: true,
	})
}

func makeSpan(tracer opentracing.Tracer, parent opentracing.SpanContext) opentracing.Span {
	span := tracer.StartSpan("benchmark_test_service", opentracing.ChildOf(parent))
	for i := 0; i < *argNumTags; i++ {
		span.SetTag(tagKeys[i], tagVals[i])
	}
	for i := 0; i < *argNumLogs; i++ {
		span.LogFields(otlog.String(logKeys[i], logVals[i]))
	}
	span.SetTag("trial", "alpha")
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
