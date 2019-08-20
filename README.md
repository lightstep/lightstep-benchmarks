# CI

lightstep-benchmarks tests the [python tracer](https://github.com/lightstep/lightstep-tracer-python) and [python cpp tracer](https://github.com/lightstep/lightstep-tracer-cpp) in two ways:

1. Regression tests are run automatically each time code is pushed.
2. Performance graphs can generated if you manually approve the jobs to run in CircleCI. More info on that [here](https://circleci.com/docs/2.0/workflows/#holding-a-workflow-for-a-manual-approval).

## Performance Graphs

Performance graphs are fairly expensive to generate, and don't have a simple pass / fail mechanism. For these reasons they aren't generated each time code is pushed automatically. They are only generated when the job "approve_make_graphs" is manually approved in CircleCI.

TODO: more info on the specific graphs which can be generated here

## Regression Tests

Regression tests are automatically run on the python and python cpp tracers each time code is pushed. The tests check the following:

- Running a tracer for 100s (emitting 500 spans per second) shouldn't use more than twice as much memory as running a tracer for 5s.
- The tracer shouldn't drop any spans if we're just sending 300 spans per second.
- A test program which generates 500 spans per second is calibrated to run at 70% CPU using a NoOp tracer. The same program shouldn't exceed 80% cpu usage when a LightStep tracer is used.
- The LightStep tracer should be able to send 3000 spans per second before the tracer (not the whole program) consumes 10% CPU.

You can also run the regression tests manually from the command-line. First, make sure that you have setup the development environment (see this section). To test the pure python tracer: `pytest --client_name python regression_tests.py`. To test the cpp python tracer: `pytest --client_name python regression_tests.py`.

# Development

Time for a brief overview of the system. The test suite is made up of three distinct parts. The **mock satellites** simulate real LightStep satellites under different conditions, the **clients** test LightStep's tracers and are implemented in various languages, and the **controller** orchestrates the tests.

## Setting up the Environment

[steps to setup the development environment]

## How to Use the Benchmark API

The Benchmark API is best illustrated with this code snippet:

```python
with Controller('[client name]') as c:
  with MockSatelliteGroup('[typical|slow_succeed|slow_fail]') as sats:
    result = c.benchmark(
      trace=True,
      satellites=sats,
      spans_per_second=100,
      runtime=10)

    # These are available no matter what.
    print(f'The test had CPU for {result.program_time} seconds')
    print(f'The test took {result.clock_time} seconds to run')
    print(f'{result.spans_sent} spans were generated during the test.')
    print(f'Percent CPU used, sampled each second: {result.cpu_list}')
    print(f'Bytes memory used, sampled each second: {result.memory_list}')
    print(f'When the test ended its memory footprint was {result.memory} bytes')

    # These are only available if a MockSatelliteGroup object is passed to
    # the controller via the satellites keyword.
    print(f'{result.spans_received} spans were received by the mock satellites.')
    print(f'{result.dropped_spans} spans were dropped.')
```

Notice that the `Controller`'s constructor needs to be passed the name of a client. These client names are the keys in the `benchmark.controller.client_args` dictionary. This dictionary holds CLI args which tell the Controller how to start different clients.

A `MockSatelliteGroup` can be started in different modes: 'typical', 'slow_success', and 'slow_fail.' 'typical' Should be used unless you know what you're doing.

# Controller

When a controller is first initialized, it will:

1.  Start a server which listens on port 8024. This server will communicate with clients and assign them work to do.
2.  Start a client which will immediately make a GET request to localhost:8024/control asking for work.
3.  Determine the behavior of the client when tracing is turned off. We will try and find:

- The nanoseconds to sleep per unit of work which leads to 70% CPU use. The controller will vary sleep per work using P control until the CPU use is within 1/2 percent of 70%.
- The units of work the client completes per second (which should be mostly independent of how many spans per second we are sending).

4.  Start up 8 mock satellites listening on ports 8360 - 8367. Mock satellites can be started with different flags, but will usually be started in 'typical' mode where their response times are typically about 1ms.

## Adding a New Client

Check out `benchmark.controller.client_args`, and you'll find that there are two clients which can

```python
# get instructions from controller
c = http_get("localhost:8024/control")
sleep_debt = 0

if c['Traced']:
  tracer = make_real_tracer()
else:
  tracer = make_mock_tracer()

for i in range(c['Repeat']):
  with tracer.start_active_span('TestSpan') as scope:
    work(c['Work'])

  # since sleep commands aren't too accurate, we save up our sleep and
  # do it all at once in a longer chunk for better accuracy
  sleep_debt += c['Sleep']
  if sleep_debt > c['SleepInterval']:
    sleep_debt -= c['SleepInterval']
    sleep(command['SleepInterval'])

# send the results of the test to the controller
http_get('localhost:8024/result', params=[spans sent, cpu use during test, test time, etc...])
```

## Garbage Collection

We have observed that in a 200 second test where 200 spans / second were sent, python runs garbage collection 49 times. The test is sufficiently long the cost of garbage collection is going to remain roughly constant across tests.

## Wire Format

```python
{
  'Trace': bool,
  'Repeat': int, # how many spans to send total
  'Work': int, # how many units of work to do / span
  'Sleep': int, # in nanoseconds
  'SleepInterval': int, # in nanoseconds
  'Exit': bool,
  'NoFlush': bool, # whether or not to call flush on tracer
  'MemoryList': list<int>, # memory footprint in bytes, taken every second
  'CPUList': list<float>, # cpu usage as decimal, taken every second
  }
```

When clients have completed the work requested in the command, they will respond with a GET to http://localhost:8023/result if the work tool 12.1 seconds to complete.

They will need to pass `ProgramTime`, `ClockTime`, and `SpansSent` key-value pairs in the query string of this get request.

## Nuances

Don't include flush time in the measurement of CPU usage.

# Ports

8023 will be the standard port for the controller to run on.
8012 - 8020 will be the standard ports for mock satellites to run on. Satellites will prefer earlier numbers, so tracers which can target only one satellite should target 8012.

```
kill `ps aux | grep mock_satellite.py | tr -s ' ' | cut -d " " -f 2 | tr '\n' ' '`
```

# Notes

up buffer size
increase reporting frequency

100ms reporting frequency
10000 spans (LS meta, trace assembler)
50000 spans (for public)

# Next Steps

- container / resource api / etc. --
  - make the clients leaner
- make a more intensive benchmarking suite on GCP

- envoy proxy (what would it take to add)
  - why is it hard?
  - duplicate sends -- same span received twice?

**why did we choose what we chose**
