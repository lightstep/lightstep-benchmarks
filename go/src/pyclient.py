#!/usr/bin/env python

import gc
import json
import select
import signal
import sys
import threading
import time
import urllib2

import opentracing
import lightstep.tracer

test_tracer = lightstep.tracer.init_tracer(
    access_token='ignored',
    secure=False,
    service_host='localhost',
    service_port=8000,
)

noop_tracer = opentracing.tracer
base_url = 'http://localhost:8000'
prime_work = 982451653
logs_memory = None
logs_size_max = 1<<20
nanos_per_second = 1e9

def prepare_logs():
    global logs_memory
    ba = bytearray(logs_size_max)
    for i in range(logs_size_max):
        ba[i] = 65 + (i % 26)
    #end
    logs_memory = bytes(ba)
#end

def do_work(n):
    x = prime_work
    while n != 0:
        # Note: Python uses arbitrary precision math, so to keep the
        # cost of this function independent of n, use the remainder
        # operation below.
	x *= prime_work
        x %= 4294967296
	n -= 1
    #end
    return x
#end

def test_body(control):
    repeat    = control['Repeat']
    sleepnano = control['Sleep']
    sleepival = control['SleepInterval']
    work      = control['Work']
    logn      = control['NumLogs']
    logsz     = control['BytesPerLog']
    answer    = None
    sleep_debt = 0  # Accumulated nanoseconds
    sleep_total = 0  # Actual sleep sum (nanoseconds)

    for i in xrange(repeat):
        span = opentracing.tracer.start_span(operation_name='span/test')
        for j in xrange(logn):
            span.log(event="testlog", payload=logs_memory[0:logsz])
        #end
        answer = do_work(work)
        span.finish()
        sleep_debt += sleepnano
        if sleep_debt <= sleepival:
            continue
        #end
        begin = time.time()
        time.sleep(sleep_debt / nanos_per_second)
        elapsed = long((time.time() - begin) * nanos_per_second)
        sleep_debt -= elapsed
        sleep_total += elapsed
    #end
    return (sleep_total / 1e9, answer)
#end

class Worker(threading.Thread):
    def __init__(self, control):
        threading.Thread.__init__(self)
        self.control = control
    #end
    def run(self):
        self.sleep_total, self.answer = test_body(self.control)
    #end
#end

def loop():
    while True:
        request = urllib2.Request(base_url + '/control')
        response = urllib2.urlopen(request)
        response_body = response.read()

        global control
        control = json.loads(response_body)

        if response.code != 200:
            raise Exception('Server returned ' + response.code)
        #end

        concurrent = control['Concurrent']
        trace      = control['Trace']

        if control['Exit']:
            sys.exit(0)
        #end

        if trace:
            opentracing.tracer = test_tracer
        else:
            opentracing.tracer = noop_tracer
        #end

        begin = time.time()
        sleep_total = 0
        answer = 0

        if concurrent == 1:
            sleep_total, answer = test_body(control)
        else:
            threads = [Worker(control) for x in xrange(concurrent)]
            for x in threads:
                x.start()
            #end
            for x in threads:
                x.join()
                sleep_total += x.sleep_total
                answer += x.answer
            #end
        #end

        end = time.time()
        flush_dur = 0.0

        if trace:
            opentracing.tracer.flush()
            flush_dur = time.time() - end
        #end

        elapsed = end - begin
        path = '/result?timing=%f&flush=%f&s=%f&a=%s' % (
            elapsed, flush_dur, sleep_total, answer)
        request = urllib2.Request(base_url + path)
        response = urllib2.urlopen(request)
        response_body = response.read()

        if response.code != 200:
            raise Exception('Server returned ' + response.code)
        #end
    #end
#end

if __name__ == '__main__':
    # Note: reference counting is sufficient for this test, no GC.
    gc.disable()

    prepare_logs()
    loop()
#end
