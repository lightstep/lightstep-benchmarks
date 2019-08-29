# Benchmarking a New Tracer with LightStep Benchmarks

This document discusses how to extend LightStep Benchmarks to benchmark tracers other than the legacy Python Tracer or the Streaming Python Tracer. LightStep Benchmarks measures tracer performance by running and monitoring instrumented client programs which use this tracer. If you want to benchmark a new tracer, you need to create a client for that tracer. Adding a new client to LightStep Benchmarks is a two step process:

1. Write a client program and add it to the clients folder (see how to do this below)
2. Add a key-value pair to the `benchmark.controller.client_args` dictionary. This dictionary maps client name to the list of command-line arguments used to start your client. (You will see that there are currently two clients registered: 'python' which tests the Python Tracer and 'python-cpp' which tests the Streaming Python Tracer).

## Writing a Client Program

Writing a client is extremely simple; just follow the pseudocode skeleton below. It is important that the client generates `c['Repeat']` spans and that `work(n)` does n floating point multiplications (or something similarly time-consuming).

```python
# The client makes an HTTP GET request to a control server which
# LightStep Benchmarks is running. The response body is a JSON formatted
# control message which contains all of the parameters of the test.
c = requests.get("localhost:8024/control").json()
sleep_debt = 0

# The client may be asked to run with a mock tracer to profile the baseline
# performance of the client program.
# Clients should report to port http://localhost:8360 since a mock Satellite
# will be listening here.
if c['Traced']:
  tracer = make_real_tracer(port=8360, host='localhost')
else:
  tracer = make_mock_tracer()

for i in range(c['Repeat']):
  with tracer.start_active_span('TestSpan') as scope:
    work(c['Work'])

    # Since sleep commands aren't too accurate, we save up our sleep and
    # do it all at once in a longer chunk for better accuracy.
    # Both c['Sleep'] and c['SleepInterval'] are in nanoseconds.
    sleep_debt += c['Sleep']
    if sleep_debt > c['SleepInterval']:
      sleep_debt -= c['SleepInterval']
      sleep(command['SleepInterval'] * 10**-9)  # nanos --> seconds

if c['Trace'] and not c['NoFlush']:
    tracer.flush()

exit()
```

You can expect that a JSON control message (in the pseudocode this was stored in the variable `c`) will look something like this:

```
{
  'Trace': False,
  'Sleep': 100,
  'SleepInterval': 100000000,
  'Work': 200000,
  'Repeat': 500,
  'NoFlush': False
}
```

## Tracer Configuration

Most of LightStep's tracers ship with very conservative defaults: they don't buffer too many spans and don't send spans to satellites very frequently. However, in production environments these conservative defaults are rarely used. Internally at LightStep, we report spans to collectors every 100ms and buffer anywhere from 10,000 to 50,000 spans. When writing a new client, it is a good idea to use these more aggressive defaults which are used in production.
