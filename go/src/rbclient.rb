require 'json'
require 'lightstep-tracer'
require 'net/http'
require 'uri'

$test_tracer = LightStep.init_global_tracer('ruby', 'ignored')

# TODO: How to get a proper NoOp tracer?
$noop_tracer = $test_tracer

$base_url = "http://localhost:8000"
$prime_work = 982451653
$logs_memory = ""
$logs_size_max = (1 << 20)
$nanos_per_second = 1e9

def prepare_logs()
  (0..$logs_size_max-1).each do |x|
    $logs_memory << ("A".ord + x%26).chr
  end
end

prepare_logs()

def do_work(n)
  x = $prime_work
  while n != 0 do
    x *= $prime_work
    x %= 4294967296
    n -= 1
  end
  return x
end

def test_body(tracer, control)
  repeat    = control['Repeat']
  sleepnano = control['Sleep']
  sleepival = control['SleepInterval']
  work      = control['Work']
  logn      = control['NumLogs']
  logsz     = control['BytesPerLog']
  sleep_debt = 0  # Accumulated nanoseconds

  (1..repeat).each do
    span = tracer.start_span('span/test')
    (1..logn).each do
      span.log("testlog", $logs_memory[0..logsz])
    end
    answer = do_work(work)
    span.finish()
    if sleepnano == 0
      next
    end
    sleep_debt += sleepnano
    if sleep_debt < sleepival
      next
    end
    before = Time.now.to_f
    sleep(sleep_debt / nanos_per_second)
    elapsed = (Time.now.to_f - before) * nanos_per_second
    sleep_debt -= elapsed
  end
end

def loop()
  while true do
    uri = URI.parse($base_url + '/control')
    resp = Net::HTTP.get(uri)
    control = JSON.parse(resp)

    concurrent = control['Concurrent']
    trace = control['Trace']

    if control['Exit']
      exit(0)
    end

    tracer = nil
    if trace
      tracer = $test_tracer
    else
      tracer = $noop_tracer
    end

    before = Time.now.to_f
    sleep_nanos = []
    answer = nil

    # TODO: Concurrency test not implemented
    sleep_nanos, answer = test_body(tracer, control)
    
    after = Time.now.to_f
    flush_dur = 0.0

    if trace
      tracer.flush()
      flush_dur = Time.now.to_f - after
    end

    elapsed = after - before
    path = sprintf('/result?timing=%f&flush=%f', elapsed, flush_dur)

    uri = URI.parse($base_url + path)
    resp = Net::HTTP.get(uri)
  end
end

loop()
