import subprocess
from http.server import BaseHTTPRequestHandler, HTTPServer
import threading
import json
import copy
from urllib.parse import urlparse, parse_qs
import time
import os
from os import path
import numpy as np
import logging

from .satellite import MockSatelliteGroup
from .utils import PROJECT_DIR

CONTROLLER_PORT = 8023
DEFAULT_SLEEP_INTERVAL = 10**8 # ns

# These values of work and repeat are starting points which should yield a
# normal number of spans / second, say 100. If you are having problems with your
# controller, change these.
CALIBRATION_WORK = 50000
CALIBRATION_REPEAT = 10000

# information about how to startup the different clients
# needs to be updates as new clients are added
client_args = {
    'python': ['python3', path.join(PROJECT_DIR, 'clients/python_client.py'), '8360', 'vanilla'],
    'python-cpp': ['python3', path.join(PROJECT_DIR, 'clients/python_client.py'), '8360', 'cpp'],
    'python-sidecar': ['python3', path.join(PROJECT_DIR, 'clients/python_client.py'), '8024', 'vanilla']
}


""" Dictionaries created by urllib.parse.parse_qs looks like {key: [value], ...}
This function take dictionaries of that format and makes them normal. """
def _format_query_json(query_dict):
    normal_dict = {}
    for key in query_dict:
        if len(query_dict[key]) == 1:
            normal_dict[key] = query_dict[key][0]
        else:
            normal_dict[key] = query_dict[key]

    return normal_dict

class CommandServer(HTTPServer):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)

        self._lock = threading.Lock()
        self._command = None
        self._result = None

    def handle_timeout(self):
        raise Exception("Client waited too long to make a request.")


    def execute_command(self, command):
        """ Queues a command to be executed. """

        with self._lock:
            assert self._command == None
            self._command = command

        while True:
            self.handle_request()

            with self._lock:
                if self._result != None:
                    result = copy.copy(self._result)
                    self._result = None
                    return result


    def next_command(self):
        with self._lock:
            assert self._command != None
            command = copy.copy(self._command)
            self._command = None

        return command

    def save_result(self, result):
        print("save result")
        assert isinstance(result, Result)

        with self._lock:
            assert self._result == None
            self._result = result


class RequestHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        parsed_url = urlparse(self.path)
        self.path = parsed_url.path # redefine path so it excludes query string
        self.query_json = _format_query_json(parse_qs(parsed_url.query))

        if self.path == "/control":
            self._handle_control()
        elif self.path == "/result":
            self._handle_result()

    def _handle_control(self):
        next_command = self.server.next_command()

        self.send_response(200)
        body_string = json.dumps(next_command)
        self.send_header("Content-Length", len(body_string))
        self.end_headers()
        self.wfile.write(body_string.encode('utf-8'))

    def _handle_result(self):
        self.send_response(200)
        self.end_headers()

        logging.info(self.query_json)
        result = Result.from_dict(self.query_json)
        self.server.save_result(result)

    def log_message(self, format, *args):
        return


''' allows us to set spans_received even after initializing ...'''
class Result:
    def __init__(self, spans_sent, program_time, clock_time, memory, memory_list, cpu_list, spans_received=0):
        self.spans_sent = spans_sent
        self.program_time = program_time
        self.clock_time = clock_time
        self.memory = memory
        self.memory_list = memory_list
        self.cpu_list = cpu_list
        self.spans_received = spans_received

    def __str__(self):
        ret = 'controller.Results object:\n'
        ret += f'\t{self.spans_per_second:.1f} spans / sec\n'
        ret += f'\t{self.cpu_usage * 100:.2f}% CPU usage\n'
        ret += f'\t{self.memory} bytes of virtual memory used at finish\n'
        if self.spans_sent > 0:
            ret += f'\t{self.dropped_spans / self.spans_sent * 100:.1f}% spans dropped (out of {self.spans_sent} sent)\n'
        ret += f'\ttook {self.clock_time:.1f}s'

        return ret

    @staticmethod
    def from_dict(result_dict, spans_received=0):
        def to_list(val):
            return val if isinstance(val, list) else [val]

        spans_sent = result_dict.get('SpansSent', 0)
        program_time = result_dict.get('ProgramTime', 0)
        clock_time = result_dict.get('ClockTime', 0)
        memory = result_dict.get('Memory', 0)
        memory_list = [int(m) for m in to_list(result_dict.get('MemoryList', []))]
        cpu_list = [float(m) for m in to_list(result_dict.get('CPUList', []))]

        return Result(int(spans_sent), float(program_time), float(clock_time), int(memory), memory_list, cpu_list, spans_received=int(spans_received))

    @property
    def spans_per_second(self):
        if self.spans_sent == 0:
            return 0
        return self.spans_sent / self.clock_time

    @property
    def dropped_spans(self):
        return self.spans_sent - self.spans_received

    @property
    def cpu_usage(self):
        return self.program_time / self.clock_time


class Controller:
    def __init__(self, client_name, target_cpu_usage=.7):
        if client_name not in client_args:
            raise Exception("Invalid client name. Did you forget to register your client?")

        self.client_startup_args = client_args[client_name]
        self.client_name = client_name

        # makes sure that the logs dir exists
        os.makedirs(path.join(PROJECT_DIR, "logs"), exist_ok=True)

        # start server that will communicate with client
        self.server = CommandServer(('', CONTROLLER_PORT), RequestHandler)
        self.server.timeout = 30 #  timeout used during client calibration
        logging.info("Started controller server.")

        self._calibrate(target_cpu_usage)

    def _calibrate(self, target_cpu_usage):
        try:
            # calibrate the amount of work the controller does so that when we are using
            # a noop tracer the CPU usage is around 70%
            self._sleep_per_work = self._estimate_sleep_per_work(target_cpu_usage)

            # calculate work per second, which we can use to estimate spans per second
            self._work_per_second = self._estimate_work_per_second()

        # if for some reason calibration fails, we still want to shutdown gracefully
        except:
            self.shutdown()
            raise

    def __enter__(self):
        return self

    def __exit__(self, type, value, traceback):
        self.shutdown()
        return False

    def shutdown(self):
        logging.info("Controller shutdown called")
        self.server.server_close() # unbind controller server from socket
        logging.info("Controller shutdown complete")


    def _estimate_work_per_second(self):
        """ Estimate how much work per second the client does. Although in practice
        this is slightly dependent on work and repeat values, it is mostly dependent on
        the sleep value.

        This varies quite a bit with spans / second, which makes it a bit tricy to get
        exactly right. Fortunately, we just have to get in the ballpark.
        """

        result = self.raw_benchmark({
            'Trace': False,
            'Sleep': CALIBRATION_WORK * self._sleep_per_work,
            'SleepInterval': DEFAULT_SLEEP_INTERVAL,
            'Exit': False,
            'Work': CALIBRATION_WORK,
            'Repeat': CALIBRATION_REPEAT,
            'NoFlush': False
        })


        assert result.clock_time > 2

        logging.info(f'Calculated that this client completes {CALIBRATION_WORK * result.spans_per_second} units of work / second.')

        return CALIBRATION_WORK * result.spans_per_second


    def _estimate_sleep_per_work(self, target_cpu_usage, trials=2):
        """ Finds sleep per work in ns which leads to target CPU usage. """
        sleep_per_work = 0

        for i in range(trials):
            # first, lets check the CPU usage without no sleeping
            result = self.raw_benchmark({
                'Trace': False,
                'Sleep': sleep_per_work * CALIBRATION_WORK,
                'SleepInterval': DEFAULT_SLEEP_INTERVAL,
                'Exit': False,
                'Work': CALIBRATION_WORK,
                'Repeat': CALIBRATION_REPEAT,
                'NoFlush': False
            })

            # make sure that client doesn't run too fast, we want a stable measurement
            assert result.clock_time > 2

            logging.info(f'clock time: {result.clock_time}, program time: {result.program_time}')

            # **assuming** that the program performs the same with added sleep commands
            # calculate the additional sleep needed throughout the program to hit the
            # target CPU usage
            additional_sleep = (result.program_time / target_cpu_usage) - result.clock_time
            sleep_per_work += (additional_sleep / (CALIBRATION_WORK * CALIBRATION_REPEAT)) * 10**9

            logging.info(f'sleep per work is now {sleep_per_work}ns')

        logging.info(f'sleep per work {sleep_per_work} yielded {result.cpu_usage * 100}% CPU usage')

        return sleep_per_work

    def benchmark(self,
            satellites=None,
            trace=True,
            no_flush=False, # we typically want flush included with our measurements
            spans_per_second=100,
            sleep_interval=DEFAULT_SLEEP_INTERVAL,
            runtime=10):

        if spans_per_second == 0:
            raise Exception("Cannot target 0 spans per second.")

        if runtime < 1:
            raise Exception("Runtime needs to be longer than 1s.")

        work = self._work_per_second / spans_per_second
        repeat = self._work_per_second * runtime / work

        # set command server timeout relative to target runtime
        self.server.timeout = runtime * 2

        # make sure that satellite span counters are all zeroed
        if satellites:
            satellites.reset_spans_received()

        result = self.raw_benchmark({
            'Trace': trace,
            'Sleep': int(work * self._sleep_per_work),
            'SleepInterval': sleep_interval,
            'Exit': False,
            'Work': int(work),
            'Repeat': int(repeat),
            'NoFlush': no_flush
        })

        # give the satellites 1s to handle the spans
        if satellites:
            time.sleep(1)
            result.spans_received = satellites.get_spans_received()
            satellites.reset_spans_received()

        return result


    def raw_benchmark(self, command):
        log_filepath = path.join(PROJECT_DIR, f'logs/{self.client_name}.log')

        # startup test process
        with open(log_filepath, 'a+') as logfile:
            logging.info("Starting client...")
            client_handle = subprocess.Popen(self.client_startup_args, stdout=logfile, stderr=logfile)
            logging.info("Client started.")

            result = self.server.execute_command(command)
            self.server.execute_command({'Exit': True})

            # at this point, we have sent the exit command and received a response
            # wait for the client program to shutdown
            logging.info("Waiting for client to shutdown...")
            while client_handle.poll() == None:
                pass
            logging.info("Client shutdown.")

        # removes results from queue
        # don't include that last result because it's from the exit command
        return result
