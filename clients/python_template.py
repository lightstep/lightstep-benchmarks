import psutil
import numpy as np
import time
import lightstep
import opentracing
import json
import sys
import requests

CONTROLLER_PORT = 8023
SATELLITE_PORT = 8012

def work(units):
    i = 1.12563
    for i in range(0, units):
        i *= i

def post_result(result):
    r = requests.post(f'http://localhost:{CONTROLLER_PORT}/result', json=result)

def perform_work(command):
    cpu_usage = []
    proc = psutil.Process()

    # if exit is set to true, end the program
    if command['exit']:
        post_result({})
        sys.exit()

    if command['trace']:
        tracer = lightstep.Tracer(
            component_name='isaac_service',
            collector_port=DEFAULT_SATELLITE_PORT,
            collector_host='localhost',
            collector_encryption='none',
            use_http=True,
            access_token='developer'
        )
    else:
        tracer = opentracing.Tracer()

    # clears the CPU usage cache so that everything we read is a result of
    # the rig
    proc.cpu_percent()

    start_time = time.time()

    # do all of the work
    for i in range(command['repeat']):
        with tracer.start_active_span('TestSpan', finish_on_close=True) as scope:
            work(command['work'])

        time.sleep(command['sleep'])

        # get CPU measurement
        cpu_usage.append(proc.cpu_percent())

    # do / don't include flush in time measurement depending on instructions
    if not command['trace']:
        test_time = time.time() - start_time
    elif command['no_flush']:
        test_time = time.time() - start_time
        tracer.flush()
    else:
        tracer.flush()
        test_time = time.time() - start_time


    mean = np.mean(cpu_usage)
    stderr = np.std(cpu_usage) / np.sqrt(len(cpu_usage))

    # TODO: might be nice to return some sort of fun object here...
    post_result({
        'mean': mean,
        'stderr':stderr,
        'test_time': test_time,
    })

if __name__ == '__main__':
    while True:
        r = requests.get(f'http://localhost:{CONTROLLER_PORT}/control')
        perform_work(r.json())
