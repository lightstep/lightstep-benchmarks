# Testing Methodology

# Parts of the System
The test suite is made up of three distinct parts. The **mock satellites** simulate real LightStep satellites under different conditions, the **clients** test LightStep's tracers and are implemented in various languages, and the **controller** orchestrates the tests.

# API

```python
with Controller('[controller name]') as c:
  with MockSatellites('typical|slow_succeed|slow_fail') as sats:
    result = c.benchmark(trace=True, satellites=sats, spans_per_second=100)

    # these are available no matter what
    result.program_time
    result.clock_time
    result.spans_sent
    result.cpu_list
    result.memory_list

    # these are only available if a satellite is passed to controller
    result.spans_received
    result.dropped_spans
```

# Controller

When a controller is first initialized, it will:

 1. Start a server which listens on port 8024. This server will communicate with clients and assign them work to do.
 2. Start a client which will immediately make a GET request to localhost:8024/control asking for work.
 3. Determine the behavior of the client when tracing is turned off. We will try and find:
   - The nanoseconds to sleep per unit of work which leads to 70% CPU use. The controller will vary sleep per work using P control until the CPU use is within 1/2 percent of 70%.
   - The units of work the client completes per second (which should be mostly independent of how many spans per second we are sending).
 4. Start up 8 mock satellites listening on ports 8360 - 8367. Mock satellites can be started with different flags, but will usually be started in 'typical' mode where their response times are typically about 1ms.

# Client

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

## Wire Format


```python
{
  'Trace': bool,
  'Repeat': int,
  'Work': int,
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



# Using the Testing Framework -- When to Stop

So you have a nice testing framework setup and you are ready to measure how spans per second influenced tracer CPU usage! But CPU usage data is riddled with random error because an OS is a complicated beast and it has lots more to do than just run a single Python program. To filter out random error, we run the same test many times -- around 50. We have observed that when the number of tests run n > 50 the CPU usage data are normally distributed. This analysis is located in a Jupyter notebook located in the analysis folder.  

The data we are collecting is CPU usage data over a time interval of 2 seconds. These data are means, since we are averaging over time. By the [Central Limit Theorem]() ...
