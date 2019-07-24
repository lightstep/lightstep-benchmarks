# Testing Methodology



# Parts of the System
The test suite is made up of three distinct parts. The **mock satellites** simulate real LightStep satellites under different conditions, the **clients** test LightStep's tracers and are implemented in various languages, and the **controller** orchestrates the tests.



# Writing Clients

1. controller creates a client
2. client makes GET request to /control
3. server responds with JSON blob that contains info about work to do
4. client does work (or may exit)
5. client makes POST request with JSON body to send test results


## Wire Format

Clients will make GET request to http://localhost:8023/control and will receive a response with a JSON body:

```python
{
  'Trace': bool,
  'Repeat': int,
  'Work': int,
  'Sleep': int, # in nanoseconds
  'Exit': bool,
  'NumLogs': int,
  'BytesPerLog': int}
```

When clients have completed the work requested in the command, they will respond with a GET to http://localhost:8023/result if the work tool 12.1 seconds to complete.

They will need to pass `ProgramTime`, `ClockTime`, and `SpansSent` key-value pairs in the query string of this get request.

## Nuances

Don't include flush time in the measurement of CPU usage. 

# Ports
8023 will be the standard port for the controller to run on.
8012 - 8020 will be the standard ports for mock satellites to run on. Satellites will prefer earlier numbers, so tracers which can target only one satellite should target 8012.

# Controller API

To use the controller API to write tests, we must use

```python



```

Note that `sleep` and `sleep_interval` are both in nanoseconds, while `test_time` is in seconds.

`c.benchmark` will return a `controller.Result` object, which has may fields:

 * spans_received
 * spans_sent
 * program_time
 * clock_time
 * spans_per_second

# Using the Testing Framework -- When to Stop

So you have a nice testing framework setup and you are ready to measure how spans per second influenced tracer CPU usage! But CPU usage data is riddled with random error because an OS is a complicated beast and it has lots more to do than just run a single Python program. To filter out random error, we run the same test many times -- around 50. We have observed that when the number of tests run n > 50 the CPU usage data are normally distributed. This analysis is located in a Jupyter notebook located in the analysis folder.  

The data we are collecting is CPU usage data over a time interval of 2 seconds. These data are means, since we are averaging over time. By the [Central Limit Theorem]() ...
