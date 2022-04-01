# LightStep Benchmarks

LightStep Benchmarks is a tool for analyzing the performance of [OpenTracing](https://opentracing.io/) Tracers. It is currently in use to measure the performance of LightStep's [legacy Python Tracer](https://github.com/lightstep/lightstep-tracer-python) and [Streaming Python Tracer](https://pypi.org/project/lightstep-native/).

This repo contains two parts: A benchmarking API which can be used to measure the performance of OpenTracing Tracers and a suite of programs which use the benchmarking API to generate performance graphs and run regression tests.

## Setup

This setup has been tested on macOS and Ubuntu.

- Install [Python 3.7](https://www.python.org/downloads/) or higher (pip will come installed by default)
- Install [Google Protobuf](https://github.com/protocolbuffers/protobuf/releases). Unfortunately, the Protobuf install is very poorly documented. As of version 3.9.1, these steps work:
  - Download and unzip the folder, then enter it
  - Run `./configure`
  - Run `sudo make install`
  - The install has succeeded if `protoc --version` prints out the Protobuf version.
- Download this repo: `git clone https://github.com/lightstep/lightstep-benchmarks.git`

Now, from the lightstep-benchmarks directory:

- (recommended) Create a virtualenv: `python3 -m venv lightbench-env` and activate it: `source lightbench-env/bin/activate`
- Install development dependencies: `pip install -r requirements-dev.txt` (you may need to use `sudo` if you choose not to set up a virtualenv)
- Generate Python Protobuf files from .proto files using this script: `./scripts/generate_proto.sh`


## Getting Started

Let's begin by benchmarking the legacy LightStep Python Tracer, since Python is already installed. We won't worry about setting up mock Satellites for the tracer to send spans to yet. First, make sure you don't have any programs bound to ports 8023 or 8360, because these port will be used during the test. You can check bound ports on Linux with: `sudo lsof -i -P -n | grep LISTEN`.

Save this code as lightstep-benchmarks/hello_world.py:

```python
from benchmark.controller import Controller

with Controller('python', target_cpu_usage=.7) as controller:
  print(controller.benchmark(
    trace=True,
    spans_per_second=100,
    runtime=10
  ))
```

From the lightstep-benchmarks directory, run `python3 hello_world.py`. You should get an output like this:

```
> controller.Results object:
>   95.3 spans / sec
>   72% CPU usage
>   100.0% spans dropped (out of 499 sent)
>   took 10.2s
```

Now let's unpack what just happened. The "python" string passed to `Controller`'s constructor tells this controller that it will be benchmarking the legacy Python Tracer. LightStep Benchmarks will test the legacy Python Tracer by running a chunk of instrumented code called a "client program" in different modes and monitoring its performance. The `target_cpu_usage=.7` keyword argument tells the controller to calibrate the Python client program so that it uses 70% of a CPU core _when a NoOp tracer is running_.

On line 4, the `benchmark` method runs a test using the specified "python" client program. Since `trace=True`, the legacy Python Tracer will be used instead of the NoOp tracer that was used for calibration. The `spans_per_second=100` and `runtime=10` arguments specify that the client program should run for about 10 seconds and generate about 100 spans per second.

The `benchmark` method returns a `Result` object, which the sample code prints (see sample code output above). As specified, the program took around ten seconds to run and send about 100 spans per second. Because we did not setup any mock Satellites for the tracer to report to, 100% of generated spans were dropped. Recall that the client program was calibrated to use 70% CPU when running a NoOp tracer. Since the test above used 72% CPU, we can assume that the LightStep Python Tracer uses 2% CPU (72% - 70%) when sending 100 spans per second.

## Benchmarking the go tracer

To benchmark the go tracer instead:
* Build the go client with `make go_client`
* In `hello_world.py`, replace the `'python'` argument to `Controller` with `'go'`
* and run `python3 hello_world.py` again.

## Further Reading

- [Tracer Benchmarking API Guide](./docs/api.md)
- [Tracer Benchmarking in LightStep's CI Pipeline](./docs/ci.md)
- [How to Benchmark a New Tracer](./docs/adding_tracers.md)

## Next Steps

- **Testing More Tracers**: It would be useful to extend this beyond the legacy Python Tracer and the Streaming Python Tracer.
- **Testing Envoy**: It would be very interesting to monitor the performance a tracer sending spans to a locally running Envoy Proxy instead of a Satellite. We predict that sending traces to a proxy would reduce the memory and CPU overhead of the Tracer.
- **Investigating Duplicate Sends**: In some conditions, Tracers might send the same span twice. It would be interesting to perform tests to check how frequently this happens.
- **Supporting OpenTelemetry**: [OpenTelemetry](https://opentelemetry.io/) is a standardized distributed tracing SDK that LightStep is leading the charge in creating. This benchmarking suite could be extended to profile the first generation of OpenTelemetry tracers.
