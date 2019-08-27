# LightStep Benchmarks

lightstep-benchmarks is a tool for analyzing the performance of [OpenTracing](https://opentracing.io/) Tracers. It is currently in use to measure the performance of LightStep's [Python Tracer](https://github.com/lightstep/lightstep-tracer-python) and [C++ / Python Tracer](https://github.com/lightstep/lightstep-tracer-cpp).

This repo contains two parts: a benchmarking API which can be used to mesure the performance of OpenTracing Tracer and a suite of programs which use the benchmarking API to generate performance graphs and run regression tests.

## Setup

This setup has only been tested on OS X and Linux (Ubuntu).

- `git clone https://github.com/lightstep/lightstep-benchmarks.git`
- Install Python 3.7 or higher and pip

Now, from the lightstep-benchmarks directory:

- Install development dependencies: `python3 -m pip install -r requirements-dev.txt`
- Install [Google Protobuf](https://github.com/protocolbuffers/protobuf/releases)
- Generate Python Protobuf files from .proto files: `./scripts/generate_proto.sh`

## Getting Started

Let's begin by benchmarking the LightStep Python Tracer, since you already have Python installed. We won't worry about setting up mock satellites for the Tracer to send spans to yet. First, make sure you don't have any programs bound to ports 8023 or 8360-8367.

Paste this code in a file in the lightstep-benchmarks directory:

```python
from benchmark.controller import Controller

with Controller('python', cpu_usage=.7) as controller:
  print(controller.benchmark(
    trace=True,
    spans_per_second=100,
    runtime=10
  ))

# > controller.Results object:
# >  95.3 spans / sec
# >  72.39% CPU usage
# >  100.0% spans dropped (out of 499 sent)
# >  took 10.2s
```

The string 'python' passed to `Controller`'s constructor tells this controller that it will be benchmarking the Python Tracer. The controller will benchmark the Python Tracer by measuring the performance of a sample program, called the Python Client Program, which is instrumented with OpenTelemetry. The `cpu_usage=.7` keyword argument tells the controller to calibrate the Python client program to use 70% of a CPU core _when a NoOp tracer is running_.

On line 4, the `benchmark` method runs a test using the same client program. Since `trace=True`, the real python LightStep tracer will be used instead of a NoOp tracer. Since a real tracer is turned on, which does more than a NoOp tracer, we should expect this test to use more than 70% CPU. The other two keyword arguments specify that the client program should run for about 10 seconds and generate about 100 spans per second. The `benchmark` method returns a `Result` object, which is printed (see sample output in comments). As we predicted, using a LightStep tracer in place of a NoOp tracer caused the Python Client to use more CPU. As expected, The program took around ten seconds to run and send about 100 spans per second. Because we did not setup any mock satellites for the Tracer to report to, 100% of generated spans were dropped.

## Further Reading

- [Benchmarking in LightStep's CI Pipeline](https://github.com/lightstep/lightstep-benchmarks/blob/master/docs/ci_integration.md)
- [How to Benchmark a New Tracer](https://github.com/lightstep/lightstep-benchmarks/blob/master/docs/adding_clients.md)
- [Benchmarking API Guide](https://github.com/lightstep/lightstep-benchmarks/blob/master/docs/api.md)
