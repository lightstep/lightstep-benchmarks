import psutil
import numpy as np
import time
import lightstep
import opentracing
import json
import sys
import requests
import argparse
import time

CONTROLLER_PORT = 8023
SATELLITE_PORT = 8012

def work(units):
    i = 1.12563
    for i in range(0, units):
        i *= i

def send_result(result):
    r = requests.get(f'http://localhost:{CONTROLLER_PORT}/result', params=result)

""" Special timer to measure process time and time spent as a result of this
process' system calls.

Records 2 * 10^-5 seconds when we immediately run start() then stop(), so tests should be
at ms scale to dwarf this contribution.
"""
class Stopwatch:
    def __init__(self):
        self.process = psutil.Process()

    """ Returns the size of process virtual memory """
    def get_memory(self):
        return self.process.memory_info()[0]

    def start(self):
        user, system, _, _ = self.process.cpu_times()
        self.start_cpu_time = user + system
        self.start_clock_time = time.time()

    def stop(self):
        user, system, _, _ = self.process.cpu_times()
        return (user + system - self.start_cpu_time, time.time() - self.start_clock_time)

def perform_work(command):
    print("performing work:", command)

    # if exit is set to true, end the program
    if command['Exit']:
        send_result({})
        sys.exit()

    if command['Trace']:
        tracer = lightstep.Tracer(
            component_name='isaac_service',
            collector_port=SATELLITE_PORT,
            collector_host='localhost',
            collector_encryption='none',
            use_http=True,
            access_token='developer'
        )
    else:
        tracer = opentracing.Tracer()


    sleep_debt = 0
    start_time = time.time()
    last_span_time = 0
    spans_sent = 0
    timer = Stopwatch()
    timer.start()

    for i in range(command['Repeat']): # time.time() < start_time + command['TestTime']:
        with tracer.start_active_span('TestSpan') as scope:
            work(command['Work'])
            spans_sent += 1

        sleep_debt += command['Sleep']

        if sleep_debt > command['SleepInterval']:
            sleep_debt -= command['SleepInterval']
            time.sleep(command['SleepInterval'] * 10**-9) # because there are 10^-9 nanoseconds / second

    memory = timer.get_memory()

    # don't include flush in time measurement
    if command['Trace'] and not command['NoFlush']:
        tracer.flush()

    cpu_time, clock_time = timer.stop()
    send_result({
        'ProgramTime': cpu_time,
        'ClockTime': clock_time,
        'SpansSent': spans_sent,
        'Memory': memory,
    })


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='Python client for LightStep Tracer benchmarking.')
    parser.add_argument('tracer', type=str, choices=["vanilla", "cpp_bindings"], help='Name of the tracer to use. Can be "vanilla" or "cpp_bindings"')
    args = parser.parse_args()

    if args.tracer == "vanilla":
        import lightstep
    if args.tracer == "cpp_bindings":
        raise Exception("Not yet implemented.")

    while True:
        r = requests.get(f'http://localhost:{CONTROLLER_PORT}/control')
        perform_work(r.json())
