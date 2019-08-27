# Adding Clients

This document discusses how to extend lightstep-benchmarks to benchmark tracers other than the Python Tracer or the C++ / Python Tracer.

---

Check out `benchmark.controller.client_args`, and you'll find that there is a client which tests the Python Tracer and a client which tests the Python / C++ Tracer. A client is a stand-alone program that sends tests spans and is instrumented with a Tracer. If you want to add a new client, you'll need to:

1. Add an entry to `benchmark.controller.client_args` which contains command-line args to start your client.
2. Write a client program and add it to the clients folder

Writing a client is extremely simple; just follow the pseudocode skeleton below. It is important that the client generates `c['Repeat']` spans and that `work(n)` does n floating point multiplications (or something similarly fast).

```python
# The client makes an HTTP get request to the control server which
# lightstep-benchmarks is running. The response body is a json formatted
# control message which contains all of the parameters of the test.
c = requests.get("localhost:8024/control").json()
sleep_debt = 0

if c['Traced']:
  tracer = make_real_tracer(port=8360, host='localhost')
else:
  tracer = make_mock_tracer()

for i in range(c['Repeat']):
  with tracer.start_active_span('TestSpan') as scope:
    work(c['Work'])

  # Since sleep commands aren't too accurate, we save up our sleep and
  # do it all at once in a longer chunk for better accuracy
  sleep_debt += c['Sleep']
  if sleep_debt > c['SleepInterval']:
    sleep_debt -= c['SleepInterval']
    sleep(command['SleepInterval'])

exit()
```

## Tracer Configuration

Most of LightStep's tracers ship with very conservative defaults: they don't buffer too many spans and don't send spans to satellites very frequently. However, in production environments such conservative defaults are rarely used. Internally at LightStep, we report spans to collectors every 100ms and buffer anywhere from 10,000 to 50,000 spans. When writing a new client, it is a good idea to use these more aggressive defaults which are used in production.
