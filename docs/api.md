# API

The Benchmark API is best illustrated with this code snippet:

```python
with Controller('[python|python-cpp]') as c:
  with MockSatelliteGroup('[typical|slow_succeed|slow_fail]') as sats:
    result = c.benchmark(
      trace=True,
      satellites=sats,
      spans_per_second=100,
      runtime=10,
      no_timeout=False,
      )

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

Notice that the `Controller`'s constructor needs to be passed the name of a client. The 'python' and 'python-cpp' clients have already been written for you. A `MockSatelliteGroup` can be started in different modes: 'typical', 'slow*success', and 'slow_fail.' 'typical' Should be used unless you read the code and know what you're doing. Since `no_timeout=False`, there will be a timeout if the test doesn't complete in `2 * runtime` seconds.

If `no_timeout=True`, there is never a timeout. Since we passed a `MockSatelliteGroup` object to `benchmark`, the `Result.spans_received` and `Result.dropped_spans` will be accurate, which was not the case in our "Getting Started" example. Keep in mind that a mock satellite group can be reused across many calls to `Controller.benchmark`.

For more detailed information about the `Controller` object, see [benchmark/controller.py](https://github.com/lightstep/lightstep-benchmarks/blob/master/benchmark/controller.py). To learn more about the `MockSatelliteGroup` object, see [benchmark/satellite.py](https://github.com/lightstep/lightstep-benchmarks/blob/master/benchmark/satellite.py). These files both have docstrings which detail the public API in depth.

## Logging

Logging is done using [Python's logging library](https://docs.python.org/3.7/library/logging.html). Logs are written to lightstep-benchmarks/logs/benchmark.log and verbose logs are written to lightstep-benchmarks/logs/benchmark_verbose.log. Warnings and errors are printed to the console by default.

## Next Steps

**Envoy Proxy**: It would be very interesting to monitor the performance a Tracer forwarding spans to an Envoy Proxy instead of a Satellite. We predict that sending traces to a local proxy would reduce the memory and CPU overhead of the Tracer.

**Duplicate Sends**: In some conditions, Tracers might send the same span twice. It would be interesting to perform tests to check how frequently this happens.

**OpenTelemetry**: [OpenTelemetry](https://opentelemetry.io/) is a standardized tracing SDK that LightStep is leading the charge in creating. This benchmarking suite could be extended to profile the next generation of Tracers written in OpenTracing.

## Architecture FAQ

> Why is this written in Python?

Python has a number of very useful modules that were used for this project: PyTest, Psutil, and Matplotlib. Python is also very easy to use cross-platform.

> Why do the Clients communicate with the Controller over HTTP?

HTTP is the simplest cross-platform method of communication. Stdout and other sorts of pipes may differ from one platform to another in annoying ways (line endings, buffering, etc.). Most modern languages also offer simple libraries which can be used to make one-line HTTP requests.

> Why is the benchmarking library bundled with regression tests and graphs into one repo?

It would be nice to have some separation between the benchmarking library and the specific way it is used: generating graphs and running regression tests on LightStep's tracers. This should be a focus of future work.
