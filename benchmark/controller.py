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

# information about how to startup the different clients
# needs to be updates as new clients are added
client_args = {
    'python': ['python3', 'clients/python_client.py', '8360', 'vanilla'],
    'python-cpp': ['python3', 'clients/python_client.py', '8360', 'cpp'],
    'python-sidecar': ['python3', 'clients/python_client.py', '8024', 'vanilla']
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

        self._command = None
        self._result = None

    def handle_timeout(self):
        raise Exception("Client waited too long to make a request.")


    def execute_command(self, command):
        """ Queues a command to be executed. """

        assert isinstance(command, Command)
        assert self._command == None

        self._command = command

        while self._result == None:
            self.handle_request()

        result = copy.copy(self._result)
        self._result = None
        return result


    def next_command(self):
        assert self._command != None

        command = copy.copy(self._command)
        self._command = None
        return command

    def save_result(self, result):
        assert isinstance(result, Result)
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
        body_string = json.dumps(next_command.to_dict())
        self.send_header("Content-Length", len(body_string))
        self.end_headers()
        self.wfile.write(body_string.encode('utf-8'))

    def _handle_result(self):
        self.send_response(200)
        self.end_headers()

        result = Result.from_dict(self.query_json)
        self.server.save_result(result)

    def log_message(self, format, *args):
        return

class Command:
    def __init__(
            self,
            trace=True,
            with_satellites=True,
            sleep=100,
            sleep_interval=10**8,
            work=1000,
            repeat=1000,
            exit=False,
            no_flush=False):

        self._trace = trace
        self._sleep = sleep
        self._sleep_interval = sleep_interval # 1ms
        self._with_satellites = with_satellites
        self._exit = exit
        self._work = work
        self._repeat = repeat
        self._no_flush = no_flush

    @staticmethod
    def exit():
        return Command(exit=True)

    @property
    def with_satellites(self):
        return self._with_satellites

    def to_dict(self):
        return {
            'Trace': self._trace,
            'Sleep': self._sleep,
            'SleepInterval': self._sleep_interval,
            'Exit': self._exit,
            'Work': self._work,
            'Repeat': self._repeat,
            'NoFlush': self._no_flush
        }


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
        logging.info("Started controller server.")

        # calibrate the amount of work the controller does so that when we are using
        # a noop tracer the CPU usage is around 70%
        self._sleep_per_work = self._estimate_sleep_per_work(target_cpu_usage)
        logging.info(f'Estimated that we need {self._sleep_per_work}ns of sleep per work to achieve {target_cpu_usage*100}% CPU usage.')

        # calculate work per second, which we can use to estimate spans per second
        self._work_per_second = self._estimate_work_per_second()
        logging.info(f'Calculated that this client completes {self._work_per_second} units of work / second.')

    def __enter__(self):
        return self

    def __exit__(self, type, value, traceback):
        self.shutdown()
        return False

    def shutdown(self):
        logging.info("Controller shutdown called")
        self.server.server_close() # unbind controller server from socket
        logging.info("Controller shutdown complete")

    """ Estimate how much work per second the client does. Although in practice
    this is slightly dependent on work and repeat values, it is mostly dependent on
    the sleep value. """
    def _estimate_work_per_second(self):
        work = 1000
        result = self.raw_benchmark(Command(
            trace=False,
            sleep=work * self._sleep_per_work,
            work=work,
            repeat=10000))

        return work * result.spans_per_second

    """ Finds sleep per work which leads to target CPU usage. """
    def _estimate_sleep_per_work(self, target_cpu_usage):
        sleep_per_work = 25
        p_constant = 10
        work = 1000

        for i in range(0, 20):
            result = self.raw_benchmark(Command(
                trace=False,
                sleep=sleep_per_work * work,
                work=work,
                repeat=5000))

            logging.info(f'${sleep_per_work:.1f}ns sleep / work --> ${result.cpu_usage * 100:.1f}% CPU usage')

            if abs(result.cpu_usage - target_cpu_usage) < .005: # within 1/2 a percent
                return sleep_per_work

            # make sure sleep per work is in range [1, 1000]
            sleep_per_work = np.clip(sleep_per_work + (result.cpu_usage - target_cpu_usage) * p_constant, 1, 1000)

        logging.warn(f'we ran a bunch of trials and could get CPU usage set to ${target_cpu_usage * 100}%')

        return sleep_per_work

    def benchmark(self,
            satellites=None,
            trace=True,
            no_flush=False, # we typically want flush included with our measurements
            spans_per_second=100,
            sleep_interval=10**7,
            runtime=10):

        if spans_per_second == 0:
            raise Exception("Cannot target 0 spans per second.")

        if runtime < 1:
            raise Exception("Runtime needs to be longer than 1s.")

        work = self._work_per_second / spans_per_second
        repeat = self._work_per_second * runtime / work

        # set command server timeout relative to runtime
        self.server.timeout = runtime * 10

        if satellites:
            satellites.reset_spans_received()

        result = self.raw_benchmark(Command(
            trace=trace,
            no_flush=no_flush,
            sleep_interval=sleep_interval,
            sleep=int(work * self._sleep_per_work),
            work=int(work),
            repeat=int(repeat)))

        if satellites:
            result.spans_received = satellites.get_spans_received()

        return result


    def raw_benchmark(self, command):
        log_filepath = path.join(PROJECT_DIR, f'logs/{self.client_name}.log')

        # startup test process
        with open(log_filepath, 'a+') as logfile:
            logging.info("Starting client...")
            client_handle = subprocess.Popen(self.client_startup_args, stdout=logfile, stderr=logfile)
            logging.info("Client started.")

            result = self.server.execute_command(command)
            self.server.execute_command(Command.exit())

            # at this point, we have sent the exit command and received a response
            # wait for the client program to shutdown
            logging.info("Waiting for client to shutdown...")
            while client_handle.poll() == None:
                pass
            logging.info("Client shutdown.")

        # removes results from queue
        # don't include that last result because it's from the exit command
        return result
