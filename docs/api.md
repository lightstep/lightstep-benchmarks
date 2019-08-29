# API Guide

## Basic Example

The full capabilities of the LightStep Benchmarks API are best illustrated with some sample code:

```python
with Controller('python', target_cpu_usage=.7) as c:
    with MockSatelliteGroup('typical') as sats:
        result = c.benchmark(
            trace=True,
            satellites=sats,
            spans_per_second=100,
            runtime=10,
            no_timeout=False)

        # These are available no matter what.
        print(f'The test had CPU for {result.program_time} seconds')
        print(f'The test took {result.clock_time} seconds to run')
        print(f'{result.spans_sent} spans were generated during the test.')
        print(f'Percent CPU used, sampled each second: {result.cpu_list}')
        print(f'Bytes memory used, sampled each second: {result.memory_list}')

        # These are only available if a MockSatelliteGroup object is passed to
        # the controller via the satellites keyword.
        print(f'{result.spans_received} spans were received by the mock satellites.')
        print(f'{result.dropped_spans} spans were dropped.')
```

The `Controller`'s constructor is passed the name of a client. To date two clients have been written: Pass 'python' to test the [legacy Python Tracer](https://github.com/lightstep/lightstep-tracer-python) or 'python-cpp' to test the [Streaming Python Tracer](https://pypi.org/project/lightstep-native/). Since `target_cpu_usage=.7` was passed to the controller, it will tune the amount of work the client program does such that it uses a baseline 70% CPU when a NoOp tracer is running.

A `MockSatelliteGroup` can be started in different modes: 'typical', 'slow_success', and 'slow_fail.' 'typical' Should be used unless you read the code and know what you're doing. The `Controller.benchmark` method is used to run a test. `trace=True` specifies that a real tracer -- in this case the legacy Python tracer -- should be used for this test. Since we passed a `MockSatelliteGroup` object to `benchmark`, the `Result.spans_received` and `Result.dropped_spans` will be accurate, which was not the case in our "Getting Started" example. A mock satellite group can be reused across many calls to `Controller.benchmark`. `spans_per_second=100` and `runtime=10` set the client program to generate around 100 spans a second and run for 10 seconds. Since `no_timeout=False`, there will be a timeout if the test doesn't complete in `2 * runtime` seconds. If instead `no_timeout=True`, there would never be a timeout.

`Controller.benchmark` returns a `Result` object. All of this object's fields are explained in the code sample.

## Satellite Disconnect Example

Mock satellite groups can be shutdown and restarted in the middle of tests. The following example shows how this can be done:

```python
RUNTIME = 90
DISCONNECT_TIME = 30
RECONNECT_TIME = 60

with Controller('python') as controller:
  satellites = MockSatelliteGroup('typical')

  def satellite_action():
    satellites.shutdown()
    time.sleep(RECONNECT_TIME - DISCONNECT_TIME)
    satellites.start('typical')

  # Shutdown then restart satellites in the middle of the test. Since
  # `benchmark` is blocking, this is done on a separate thread.
  shutdown_timer = Timer(DISCONNECT_TIME, satellite_action)
  shutdown_timer.start()

  # Start the test
  print(controller.benchmark(
      trace=True,
      spans_per_second=100,
      runtime=RUNTIME,
  ))

  # Shutdown satellites after the test is finished
  satellites.shutdown()
```

For more detailed information about the `Controller` object, see [benchmark/controller.py](https://github.com/lightstep/lightstep-benchmarks/blob/master/benchmark/controller.py). To learn more about the `MockSatelliteGroup` object, see [benchmark/satellite.py](https://github.com/lightstep/lightstep-benchmarks/blob/master/benchmark/satellite.py). These files both have docstrings which explain the public API in depth.

## Calibration

When a controller object is created, it first calibrates the client it was told to observe. For the sake of example, let's assume that `target_cpu_usage=.7` was passed to this controller. The controller is first going to determine the behavior of the client when a NoOp tracer is being used. The controller will characterize the client's behavior by computing two constants:

1. **target sleep / work ratio**: The nanoseconds of sleep per unit of work which leads to 70% CPU use. The controller finds this using a form of proportional control.
2. **work / second**: The units of work the client completes per second when using 70% CPU. The controller finds this by running one test.

We use the sleep / work ratio to calculate 'Sleep' from 'Work' for all future tests. By using this constant sleep / work ratio, when a NoOp tracer is used the client program will always use around 70% CPU no matter the other parameters of the test. The second of these constants is used to convert `runtime=10` and `spans_per_second=100` keyword arguments passed to `Controller.benchmark` into 'Spans' and 'Repeat' test parameters which are sent to the client.

## Logging

LightStep benchmarks handles logging using [Python's logging library](https://docs.python.org/3.7/library/logging.html). Logs are written to lightstep-benchmarks/logs/benchmark.log and verbose logs are written to lightstep-benchmarks/logs/benchmark_verbose.log. Warnings and errors are printed to the console by default.

## Architecture FAQ

> Why is this written in Python?

Python has a number of very useful modules that were used for this project: PyTest, Psutil, and Matplotlib. Python is also very easy to use cross-platform.

> Why do the clients communicate with the Controller over HTTP?

HTTP is the simplest cross-platform method of communication. STDOUT and other sorts of pipes may differ from one platform to another in annoying ways (line endings, buffering, etc.). Most modern languages also offer simple libraries which can be used to make one-line HTTP requests. It's also nice to use STDOUT
and STDERR to report info and error logs.

> Why is the benchmarking library bundled with regression tests and graphs into one repo?

It would be nice to have some separation between the benchmarking library and the specific way it is used: generating graphs and running regression tests on LightStep's tracers. This should be a focus of future work.
