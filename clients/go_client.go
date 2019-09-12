package main

import (
  "github.com/opentracing/opentracing-go"
  otlog "github.com/opentracing/opentracing-go/log"
  "github.com/lightstep/lightstep-tracer-go"
  "flag"
  "time"
  "math"
  "context"
)

const (
  satellitePort = 8360
  reportingPeriod = 200 * time.Millisecond
  maxBufferedSpans = 10000
  spansPerLoop = 6
)

var argTracer = flag.String("tracer", "", "Which LightStep tracer to use")
var argTrace = flag.Int("trace", 0, "Whether to trace")
var argSleep = flag.Float64("sleep", 0, "The amount of time to sleep for eadh span")
var argSleepInterval = flag.Int("sleep_interval", 0, "The duration of each sleep")
var argWork = flag.Int("work", 0, "The quanitity of work to perform between spans")
var argRepeat = flag.Int("repeat", 0, "The number of span generation repetitions to perform")
var argNoFlush = flag.Int("no_flush", 0, "Whether to flush on finishing")

var workResult = 0.0

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
  for i:=0; i<units; i++ {
    x *= math.Sqrt(math.Log(float64(i)))
  }
  workResult = x
}

func buildTracer() opentracing.Tracer {
  if *argTrace == 0 {
    return opentracing.NoopTracer {}
  }
  return lightstep.NewTracer(lightstep.Options{
    AccessToken:"developer",
    UseHttp: true,
    Tags: map[string]interface{}{
      lightstep.ComponentNameKey: "isaac_service",
    },
    Collector: lightstep.Endpoint{
      Host: "127.0.0.1",
      Port: satellitePort,
      Plaintext: true,
    },
    ReportingPeriod: reportingPeriod,
    MaxBufferedSpans: maxBufferedSpans,
  })
}

func generateSpans(tracer opentracing.Tracer, unitsWork int, numSpans int, parent opentracing.SpanContext) {
  client_span := tracer.StartSpan("make_some_request", opentracing.ChildOf(parent))
  defer client_span.Finish()
  client_span.SetTag("http.url", "http://somerequesturl.com")
  client_span.SetTag("http.method", "POST")
  client_span.SetTag("span.kind", "client")
  doWork(unitsWork)
  numSpans -= 1
  if numSpans == 0 {
    return
  }

  server_span := tracer.StartSpan("handle_some_request", opentracing.ChildOf(client_span.Context()))
  defer server_span.Finish()
  server_span.SetTag("http.url", "http://somerequesturl.com")
  server_span.SetTag("span.kind", "server")
  server_span.LogFields(
    otlog.String("event", "soft error"),
    otlog.String("message", "some cache missed :("),
  )
  doWork(unitsWork)
  numSpans -= 1
  if numSpans == 0 {
    return
  }

  db_span := tracer.StartSpan("database_write", opentracing.ChildOf(server_span.Context()))
  defer db_span.Finish()
  db_span.SetTag("db.user", "test_user")
  db_span.SetTag("db.type", "sql")
  db_span.SetTag("db_statement",
    "UPDATE ls_employees SET email = 'isaac@lightstep.com' WHERE employeeNumber = 27;")
  db_span.SetTag("error", true)
  db_span.LogFields(
    otlog.String("event", "error"),
    otlog.String("stack", 
               `File \"example.py\", line 7, in <module>
caller()
File \"example.py\", line 5, in caller
callee()
File \"example.py\", line 2, in callee
raise Exception(\"Yikes\")`))
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
    spansToSend := min(*argRepeat - spansSent, spansPerLoop)
    generateSpans(tracer, *argWork, spansToSend, nil)
    spansSent += spansToSend
    sleepDebt += *argSleep*float64(spansToSend)
    if sleepDebt > float64(*argSleepInterval) {
      sleepDebt -= float64(*argSleepInterval)
      time.Sleep(time.Duration(*argSleepInterval)*time.Nanosecond)
    }
  }
  if *argTrace != 0 && *argNoFlush != 1 {
    tracer.(lightstep.Tracer).Close(context.Background())
  }
}

func main() {
  flag.Parse()
  performWork()
}
