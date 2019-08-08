import psutil
import numpy as np
import time
import opentracing
import json
import sys
import requests
import argparse
import time
import gc
import math

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
class Monitor:
    def __init__(self):
        self.process = psutil.Process()

    def get_memory(self):
        """ Returns the size of process virtual memory """
        return self.process.memory_info()[0]

    def start(self):
        user, system, _, _ = self.process.cpu_times()
        self.start_cpu_time = user + system
        self.start_clock_time = time.time()
        self.get_cpu()

    def get_cpu(self):
        """ Gets CPU %, calculated since last call to split. """
        return self.process.cpu_percent(interval=None)

    def stop(self):
        user, system, _, _ = self.process.cpu_times()
        return (user + system - self.start_cpu_time, time.time() - self.start_clock_time)

def build_tracer(command, tracer_name, port):
    if command['Trace'] and tracer_name == "vanilla":
        print("We're using the python tracer.")
        import lightstep
        return lightstep.Tracer(
            component_name='isaac_service',
            collector_port=port,
            collector_host='localhost',
            collector_encryption='none',
            use_http=True,
            access_token='developer'
        )
    elif command['Trace'] and tracer_name == "cpp":
        print("We're using the python-cpp tracer.")
        import lightstep_native
        return lightstep_native.Tracer(
            component_name='isaac_service',
            access_token='developer',
            use_stream_recorder=True,
            collector_plaintext=True,
            satellite_endpoints=[{'host':'localhost', 'port':p} for p in range(port, port + NUM_SATELLITES)],
        )

    print("We're using a NoOp tracer.")
    return opentracing.Tracer()

SPANS_PER_JUMP = 3
JUMPS = 2
SPANS_PER_REPEAT = SPANS_PER_JUMP * JUMPS

def generate_spans(tracer, work_list, scope=None):
    """
    :work_list: the amount of work to do at each hop
    """
    if not work_list:
        return

    with tracer.start_span(operation_name='make_some_request', child_of=scope) as client_span:
        tracer.scope_manager.activate(client_span, True)
        client_span.set_tag('http.url', 'http://somerequesturl.com')
        client_span.set_tag('http.method', "POST")
        client_span.set_tag('span.kind', 'client')

        with tracer.start_span(operation_name='handle_some_request', child_of=client_span) as server_span:
            tracer.scope_manager.activate(server_span, True)
            server_span.set_tag('http.url', 'http://somerequesturl.com')
            server_span.set_tag('span.kind', 'server')

            server_span.log_kv({'event': 'cache_miss', 'message': 'some cache hit and so we didn\'t have to do extra work'})

            with tracer.start_span(operation_name='database_write', child_of=server_span) as db_span:
                tracer.scope_manager.activate(db_span, True)
                db_span.set_tag('db.user', 'test_user')
                db_span.set_tag('db.type', 'sql')
                db_span.set_tag('db_statement', 'UPDATE ls_employees SET email = \'isaac@lightstep.com\' WHERE employeeNumber = 27;')

                # pretend that an error happened
                db_span.set_tag('error', True)
                db_span.log_kv({'event': 'error', 'stack': "File \"example.py\", line 7, in \<module\>\ncaller()\nFile \"example.py\", line 5, in caller\ncallee()\nFile \"example.py\", line 2, in callee\nraise Exception(\"Yikes\")\n"})

            work(work_list[0])
            generate_spans(tracer, work_list[:-1], scope=server_span)

def perform_work(command, tracer_name, port):
    print("**********")
    print("performing work:", command, tracer_name, port)

    tracer = build_tracer(command, tracer_name, port)

    # if exit is set to true, end the program
    if command['Exit']:
        send_result({})
        print("sent exit response, now exiting...")
        sys.exit()

    sleep_debt = 0
    start_time = time.time()
    spans_sent = 0

    last_memory_save = time.time()
    memory_list = []
    cpu_list = []

    monitor = Monitor()
    monitor.start()


    for i in range(int(command['Repeat'] / SPANS_PER_REPEAT)):
        # each hop genereates 3 spans and we do 2 hops
        generate_spans(tracer, [command['Work'], command['Work']])
        spans_sent += SPANS_PER_REPEAT
        sleep_debt += command['Sleep'] * SPANS_PER_REPEAT

        if sleep_debt > command['SleepInterval']:
            sleep_debt -= command['SleepInterval']
            time.sleep(command['SleepInterval'] * 10**-9) # because there are 10^-9 nanoseconds / second

        if time.time() > last_memory_save + MEMORY_PERIOD:
            memory_list.append(monitor.get_memory())
            # saves CPU percentage as fraction since last call
            cpu_list.append(monitor.get_cpu() / 100)
            last_memory_save = time.time()

    memory = monitor.get_memory()

    # don't include flush in time measurement
    if command['Trace'] and not command['NoFlush']:
        print("flushing")
        tracer.flush()

    cpu_time, clock_time = monitor.stop()

    print("sending result")
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
