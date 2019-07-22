The **controller** (controller.py) will be a server which clients will hit with requests for work. The controller will respond with descriptions of work to do.

1. controller creates a client
2. client makes GET request to /control
3. server responds with JSON blob that contains info about work to do
4. client does work (or may exit)
5. client makes POST request with JSON body to send test results

# Wire Format
## Getting Control Command
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

When clients have completed the work requested in the command, they will respond with a GET to http://localhost:8023/result?Timing=12.1 if the work tool 12.1 seconds to complete.


# Ports
8023 will be the standard port for the controller to run on.
8012 - 8020 will be the standard ports for mock satellites to run on. Satellites will prefer earlier numbers, so tracers which can target only one satellite should target 8012.

# Controller API

The controller has a really simple interface that you can use to design your own tests!

The entire API uses the controller class: `c = controller.Controller(["python3", "python_client.py"])`. Once you have a controller object, you can benchmark using `c.benchmark(command)`. Command is an instance of the `controller.Command` class.



```python
controller.Command(spans_per_second, trace=True, sleep=10**5, sleep_interval=10**7, test_time=5, with_satellite=True)
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
