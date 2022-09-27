package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	spansPerLoop = 6
)

var argTrace = flag.Int("trace", 0, "Whether to trace")
var argSleep = flag.Float64("sleep", 0, "The amount of time to sleep for each span")
var argSleepInterval = flag.Int("sleep_interval", 0, "The duration of each sleep")
var argWork = flag.Int("work", 0, "The quanitity of work to perform between spans")
var argRepeat = flag.Int("repeat", 0, "The number of span generation repetitions to perform")
var argNoFlush = flag.Int("no_flush", 0, "Whether to flush on finishing")
var argNumTags = flag.Int("num_tags", 0, "The number of tags to set on a span")
var argNumLogs = flag.Int("num_logs", 0, "The number of logs to set on a span")

var workResult = 0.0
var attributes []attribute.KeyValue = nil
var events []string = nil

func setupAnnotations() {
	// setup attributes
	attributes = make([]attribute.KeyValue, *argNumTags)
	for i := 0; i < *argNumTags; i++ {
		attributes[i] = attribute.String(fmt.Sprintf("tag.key%d", i), fmt.Sprintf("tag.valie%d", i))
	}
	// setup logs
	events = make([]string, *argNumLogs)
	for i := 0; i < *argNumLogs; i++ {
		events[i] = fmt.Sprintf("log.key%d:log.value%d", i, i)
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

func buildTracer() trace.Tracer {
	if *argTrace == 0 {
		return trace.NewNoopTracerProvider().Tracer("OtelBenchmark")
	}
	client := otlptracehttp.NewClient(
		otlptracehttp.WithEndpoint("satellite-otel-go:8360"),
		otlptracehttp.WithInsecure(),
	)
	exporter, _ := otlptrace.New(context.Background(), client)

	r, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("ExampleService"),
		),
	)

	if err != nil {
		panic(err)
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(r),
	).Tracer("OtelBenchmark")
}

func makeSpan(ctx context.Context, tracer trace.Tracer) (context.Context, trace.Span) {
	ctx, span := tracer.Start(ctx, "benchmark_test_service")
	span.SetAttributes(attributes...)
	for i := 0; i < *argNumLogs; i++ {
		span.AddEvent(events[i])
	}
	// span.SetTag("trial", "alpha")
	return ctx, span
}

func generateSpans(ctx context.Context, tracer trace.Tracer, unitsWork int, numSpans int) {
	ctx, client_span := makeSpan(ctx, tracer)
	defer client_span.End()
	doWork(unitsWork)
	numSpans -= 1
	if numSpans == 0 {
		return
	}

	ctx, server_span := makeSpan(ctx, tracer)
	defer server_span.End()
	doWork(unitsWork)
	numSpans -= 1
	if numSpans == 0 {
		return
	}

	ctx, db_span := makeSpan(ctx, tracer)
	defer db_span.End()
	doWork(unitsWork)
	numSpans -= 1
	if numSpans == 0 {
		return
	}

	generateSpans(ctx, tracer, unitsWork, numSpans)
}

func performWork() {
	tracer := buildTracer()

	sleepDebt := 0.0
	spansSent := 0
	for spansSent < *argRepeat {
		spansToSend := min(*argRepeat-spansSent, spansPerLoop)
		generateSpans(context.Background(), tracer, *argWork, spansToSend)
		spansSent += spansToSend
		sleepDebt += *argSleep * float64(spansToSend)
		if sleepDebt > float64(*argSleepInterval) {
			sleepDebt -= float64(*argSleepInterval)
			time.Sleep(time.Duration(*argSleepInterval) * time.Nanosecond)
		}
	}
}

func main() {
	flag.Parse()
	setupAnnotations()
	performWork()
}
