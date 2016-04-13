#!/usr/local/bin/python -u

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

global always
always = True

timer = time.time

# TODO generate constants from benchlib/bench.go and import them

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

def prepare_logs():
    global logs_memory
    ba = bytearray(logs_size_max)
    for i in range(logs_size_max):
        ba[i] = i % 256
    #end
    logs_memory = bytes(ba)
#end

def do_work(n):
    x = prime_work
    while n >= 0:
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
    sleep     = control['Sleep'] / 1e9
    sleepival = control['SleepInterval'] / 1e9
    work      = control['Work']
    logn      = control['NumLogs']
    logsz     = control['BytesPerLog']
    sleeper   = Sleeper(sleepival)
    for i in xrange(repeat):
        span = opentracing.tracer.start_span(operation_name='span/test')
        for j in xrange(logn):
            span.log_event(logs_memory[0:logsz])
        #end
        x = do_work(work)
        if not always:
            print x  # Prevent dead code elimination
        #end
        span.finish()
        if sleep != 0.0:
            sleeper.amortized_sleep(sleep)
        #end
    #end
    sleeper.sleep()
#end

class Sleeper:
    def __init__(self, interval):
        self.sleep_debt = 0.0
        self.sleep_interval = interval
    #end

    def amortized_sleep(self, duration):
        self.sleep_debt += duration

        if self.sleep_debt >= self.sleep_interval:
            self.sleep()
        #end
    #end

    def sleep(self):
        if self.sleep_debt <= 0:
            return
        #end
        begin = timer()
        while self.sleep_debt > 0.0:
            time.sleep(self.sleep_debt)
            now = timer()
            self.sleep_debt -= (now - begin)
            begin = now
        #end
    #end
#end

class Worker(threading.Thread):
    def __init__(self, control):
        threading.Thread.__init__(self)
        self.control = control
    #end
    def run(self):
        test_body(self.control)
    #end
#end

def loop():
    while True:
        request = urllib2.Request(base_url + '/control')
        response = urllib2.urlopen(request)
        response_body = response.read()

        control = json.loads(response_body)

        if response.code != 200:
            raise Exception('Server returned ' + response.code)
        #end

        concurrent = control['Concurrent']
        trace      = control['Trace']
        noflush    = control['NoFlush']

        if control['Exit']:
            sys.exit(0)
        #end

        if trace:
            opentracing.tracer = test_tracer
        else:
            opentracing.tracer = noop_tracer
        #end

        begin = timer()

        if concurrent == 1:
            test_body(control)
        else:
            threads = [Worker(control) for x in xrange(concurrent)]
            [x.start() for x in threads]  # TODO is this legitimate style?
            [x.join() for x in threads]
        #end

        if trace and not noflush:
            opentracing.tracer.flush()
        #end
        
        elapsed = timer() - begin

        request = urllib2.Request(base_url + '/result?timing=%f' % elapsed)
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
