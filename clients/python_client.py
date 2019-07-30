import psutil
import numpy as np
import time
import opentracing
import json
import sys
import requests
import argparse
import time

MEMORY_PERIOD = 1 # report memory use every 5 seconds
CONTROLLER_PORT = 8023
NUM_SATELLITES = 8

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
        self.split()

    def split(self):
        """ Gets CPU %, calculated since last call to split. """
        return self.process.cpu_percent(interval=None)

    def stop(self):
        user, system, _, _ = self.process.cpu_times()
        return (user + system - self.start_cpu_time, time.time() - self.start_clock_time)

""" Mode is either vanilla or cpp_bindings. """
def perform_work(command, tracer_name, port):
    print("performing work:", command)

    # if exit is set to true, end the program
    if command['Exit']:
        send_result({})
        sys.exit()

    if command['Trace'] and tracer_name == "vanilla":
        import lightstep
        tracer = lightstep.Tracer(
            component_name='isaac_service',
            collector_port=port,
            collector_host='localhost',
            collector_encryption='none',
            use_http=True,
            access_token='developer'
        )
    elif command['Trace'] and tracer_name == "cpp":
        import lightstep_native
        tracer = lightstep_native.Tracer(
            component_name='isaac_service',
            access_token='developer',
            use_stream_recorder=True,
            collector_plaintext=True,
            satellite_endpoints=[{'host':'localhost', 'port':p} for p in range(port, port + NUM_SATELLITES)],
        )
    else:
        tracer = opentracing.Tracer()


    sleep_debt = 0
    start_time = time.time()
    spans_sent = 0

    last_memory_save = time.time()
    memory_list = []
    cpu_list = []

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

        if time.time() > last_memory_save + MEMORY_PERIOD:
            memory_list.append(timer.get_memory())
            # saves CPU percentage as fraction since last call
            cpu_list.append(timer.split() / 100)
            last_memory_save = time.time()

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
        'MemoryList': memory_list,
        'CPUList': cpu_list,
    })


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='Start a client to test a LightStep tracer.')
    parser.add_argument('port', type=int, help='Which port to connect to the satellite on.')
    parser.add_argument('tracer', type=str, choices=["vanilla", "cpp"], help='Which LightStep tracer to use.')
    args = parser.parse_args()

    while True:
        r = requests.get(f'http://localhost:{CONTROLLER_PORT}/control')
        perform_work(r.json(), args.tracer, args.port)
