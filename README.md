# LightStep Client Benchmarks

## Experiment Design

The benchmark suite consists of a small test program in each
language executed by the test controller.  The LightStep client
library connects to the controller acting as a LightStep
Collector (using one of several protocols), and the test program
connects to the controller using a simple REST API for the
control interface.  The test program enters a loop in which it
repeatedly:

1. Requests a Control struct, defining a traced or untraced operation
- Executes the defined operation, possibly recording a Span
- Responds with the client-observed walltime

In pseudocode, each test program exercises the client library as
follows:

```
C := the control struct
for i := 0; i < C.Repeat; i++ {
  span := tracer.startSpan()
  for j := 0; j < C.Work; j++ {
    do_some_busy_work
  }
  span.finish()
  sleep(C.Sleep)
}
```

The Control fields are designed to support several kinds of
experiment, discussed below.  The Control fields are:

- `Work` the number of units of work to perform per operation, where each unit
is an arbitrary but brief, fixed-CPU-cost operation to perform
- `Trace` is true if the operation is traced by a LightStep tracer (vs a No-Op tracer)
- `Sleep` a number of nanoseconds to sleep after completing the work
- `SleepInterval` a number of nanoseconds for amortized sleep
- `SleepCorrection` an estimated number of nanoseconds the controller expects sleep to exceed by, useful in runtimes without sleep support (i.e., nodejs)
- `Repeat` the number of times to repeat the operation
- `NoFlush` is true indicating not to flush the Tracer after the operations (i.e., to exclude flush time from the measurement)
- `Concurrent` is the number of concurrent threads / routines / tasks performing these `Repeat` operations each
- `NumLogs` is the number of log statements per Span
- `BytesPerLog` is the number of bytes per log
- `Exit` is true indicating to exit the test program immediately

After running the operation, the client responds with a Result structure containing:

- `Timing` the number of seconds (walltime) the client observed to complete the operations and (maybe) flush the spans
- `Sleeps` (Optional, for setting `SleepCorrection` in some runtimes) a comma-separated timeseries of the observed walltime of sleep operations, in nanoseconds
- `Answer` (Optional) the result of the operation (to avoid optimization)

### Calibration

The controller process runs the client program through a number
of preliminary tests to measure testing overhead.  We are
interested in knowing:

- The nanoseconds per unit of work
- The fixed cost of an untraced, no-work operation--this
  corresponds to the testing overhead of each loop iteration.
- The fixed cost of an traced, no-work operation--this
  corresponds to the synchronous cost (i.e., not deferred) of
  empty span creation

These constants will be used to correct the observed measurements
for CPU time lost to the test itself.

#### Sleep Calibration

The CPU saturation test described below sets up an experiment in
which the worker alternatingly performs busy work and then
sleeps, where the ratio of work to sleep models the system
utilization factor.  By sleeping, the deferred work of sending
spans to the collector is interleaved with the test operations.

Since very short sleep operations are not reliable on most
runtimes, sleeps are amortized into larger intervals, on the
order of 10s of milliseconds (see `Control.SleepInterval`).
Sleep operations are still not reliable on runtimes with a
single-threaded event loop (e.g., nodejs), so further correction
is needed.  Test programs with unreliable sleep operations are
required to report the measured sleep duration (walltime) back to
the controller via `Result.Sleeps`.

#### Empty Span Cost

The most important cost, from the users perspective, is the fixed
cost of an empty, traced operation, since it approximates the
synchronous impact to the calling thread, which is often on a
critical path for latency.

Appropraiate values for this measurement are in the 1-100μs
range.  We consider 10μs a good measurement, while a measurement
of 100μs indicates a library that needs improvement.  Current
measurements:

- golang 0.5μs
- nodejs 10μs
- python 15μs

Our goal is that unless the CPU is saturated, the empty span cost
is the only additional user-perceived latency impact that results
from tracing.

### Total CPU Cost

The goal of this experiment is to estimate the worst-case CPU
usage of all LightStep client-related activities, including the
cost of span creation in the caller's thread, _plus all deferred
work inside the client library_.

The test controller will repeat an experiment for several values
along each of the following dimensions:

- `qps` how many Spans are generated per second (per `Concurrent` thread)
- `load` what is the load factor (per `Concurrent` thread)

The test method runs experiments for increasing load factors at
each desired qps, for every client.  The test Control objects
have their `Work`, `Sleep`, and `Repeat` values set to execute a
specific load factor for a fixed period of time.  For example,
assume the Work function takes 1ns per unit of work (by
calibration), then to setup a 100qps test at 70% load, use the
following settings to run for 30 seconds:

- Control.Work == 7000000 (7 million nanoseconds == 7ms)
- Control.Sleep == 3000000 (3 million nanoseconds == 3ms)
- Control.Repeat == 3000 (30 seconds * 100 qps)

The test program should have its number of CPUs hard-limited to
the `Concurrent` factor, to ensure there are no extra CPU
resources available.  As a result, when the test runs for the
specified amount of time, we can be sure the CPU is not
saturated.

When the test program runs for longer than the specified amount
of time, its sleep interval should be reduced until the test runs
for the specified amount of time, since increasing the sleep
interval gives the deferred work more time to run without
changing qps.

When the test program runs for longer than the specified amount
of time _and_ with zero sleeping, the CPU is saturated at the
given qps and we can compute the tracing "tax" in terms of CPU
impairment.  For example, if the system reaches saturation at a
load factor of 98%, we have observed a 2% tax or that the CPU is
impaired by 2%.

By comparing the saturation point with and without tracing, we
can separate tracing costs from other overheads in the system.
This process will be repeated to construct a plot of CPU overhead
vs. qps for every client library.

Preliminary data:

- golang @100qps 0%; @250 qps < 0.1%; @500qps < 0.2%; @750qps < 0.3%; @1000qps < 0.5%
- python in progress following recent speedups, CPU-limits required for final measurements
- nodejs unfinished, sleep correction logic needed

Note: Because Python performance is showing glaring problems, I
will prioritize fixing the Python client ahead of completing
these measurements.

Note: The numbers above are preliminary; longer tests will be needed
for greater precision.

### Concurrency Costs

TODO planned work

How much self-interference does the LightStep client library
have?  The Total CPU Cost tests will be repeated for higher
concurrency factors, to estimate the scaling function.

### Rate Limit Config

At what empty span rate does the client begins dropping spans?

- python observed @ 500
- nodejs not observed @ 1000
- golang not observed @ 1000

### Network Costs

What is the outbound network cost per empty Span?

XXX TODO these are payload sizes, not network sizes

- nodejs 290 bytes
- python 155 bytes
- golang 125 bytes

### Logging Costs

TODO Implemented in Python only. The effect in Python is to inrease the span throughput, ironically, by slowing down the calling thread. Bad!

How much CPU cost is there for one 0-byte logging statement?  
How much CPU cost is there for each additional byte of log message?

### Tag Costs

TODO This was not studied.

How much CPU cost is there for each key:value entry?  
How much cost is there for each byte of key:value entry?
