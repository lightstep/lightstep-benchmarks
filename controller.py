import subprocess
from http.server import BaseHTTPRequestHandler, HTTPServer
import threading
import json
import copy
from urllib.parse import urlparse, parse_qs
from satellite.controller import MockSatelliteGroup
import time
import os
import numpy as np
import logging

LONGEST_TEST = 100
CONTROLLER_PORT = 8023
SATELLITE_PORT = 8360


""" Dictionaries created by urllib.parse.parse_qs looks like {key: [value], ...}
This function take dictionaries of that format and makes them normal. """
def _format_query_json(query_dict):
    normal_dict = {}
    for key in query_dict:
        normal_dict[key] = query_dict[key][0]

    return normal_dict

class CommandServer(HTTPServer):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)

        self._commands = []
        self._results = []

    def handle_timeout(self):
        raise Exception("Client waited too long to make a request.")

    """ Queues a command to be executed. """
    def add_command(self, command):
        assert isinstance(command, Command)
        self._commands.append(command)

    """ Pops the next command from the queue. """
    def next_command(self):
        if len(self._commands) == 0:
            return None

        return self._commands.pop(0)

    def length_results(self):
        return len(self._results)

    def pop_results(self):
        results = copy.deepcopy(self._results)
        self._results = []
        return results

    def add_result(self, result):
        assert isinstance(result, Result)
        self._results.append(result)


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

        if not next_command:
            logging.error("Client requested a command, but no more commands were available.")
            return

        self.send_response(200)
        body_string = json.dumps(next_command.to_dict())
        self.send_header("Content-Length", len(body_string))
        self.end_headers()
        self.wfile.write(body_string.encode('utf-8'))

    def _handle_result(self):
        self.send_response(200)
        self.end_headers()

        result = Result.from_dict(self.query_json)
        self.server.add_result(result)

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
        return Command(0, exit=True)

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
    def __init__(self, spans_sent, program_time, clock_time, memory, spans_received=0):
        self._spans_sent = spans_sent
        self._program_time = program_time
        self._clock_time = clock_time
        self._memory = memory
        self.spans_received = spans_received

    def __str__(self):
        ret = 'controller.Results object:\n'
        ret += f'\t{self.spans_per_second:.1f} spans / sec\n'
        ret += f'\t{self.cpu_usage * 100:.2f}% CPU usage\n'
        ret += f'\t{self.memory} bytes of virtual memory used\n'
        if self.spans_sent > 0:
            ret += f'\t{self.dropped_spans / self.spans_sent * 100:.1f}% spans dropped (out of {self.spans_sent} sent)\n'
        ret += f'\ttook {self.clock_time:.1f}s'

        return ret

    @staticmethod
    def from_dict(result_dict, spans_received=0):
        spans_sent = result_dict.get('SpansSent', 0)
        program_time = result_dict.get('ProgramTime', 0)
        clock_time = result_dict.get('ClockTime', 0)
        memory = result_dict.get('Memory', 0)

        return Result(int(spans_sent), float(program_time), float(clock_time), int(memory), spans_received=int(spans_received))

    @property
    def memory(self):
        return self._memory

    @property
    def spans_per_second(self):
        if self.spans_sent == 0:
            return 0
        return self.spans_sent / self.clock_time

    @property
    def program_time(self):
        return self._program_time

    @property
    def clock_time(self):
        return self._clock_time

    @property
    def spans_sent(self):
        return self._spans_sent

    @property
    def dropped_spans(self):
        return self.spans_sent - self.spans_received

    @property
    def cpu_usage(self):
        return self.program_time / self.clock_time



class Controller:
    def __init__(self, client_startup_args, client_name='client', target_cpu_usage=.7, num_satellites=1):
        self.client_startup_args = client_startup_args
        self.client_name = client_name

        # can be 'typical', 'slow', 'no_response'
        self._satellite_mode = 'typical'
        self._satellite_ports = list(range(SATELLITE_PORT, SATELLITE_PORT + num_satellites))

        # makes sure that the logs dir exists
        os.makedirs("logs", exist_ok=True)

        # start server that will communicate with client
        self.server = CommandServer(('', CONTROLLER_PORT), RequestHandler)
        logging.info("Started controller server.")

        # server timeout is twice the longest test length
        self.server.timeout = LONGEST_TEST * 2

        # calibrate the amount of work the controller does so that when we are using
        # a noop tracer the CPU usage is around 70%
        self._sleep_per_work = self._estimate_sleep_per_work(target_cpu_usage)
        logging.info(f'Estimated that we need {self._sleep_per_work}ns of sleep per work to achieve {target_cpu_usage*100}% CPU usage.')

        # calculate work per second, which we can use to estimate spans per second
        self._work_per_second = self._estimate_work_per_second()
        logging.info(f'Calculated that this client completes {self._work_per_second} units of work / second.')
        logging.info("Controller has initialized.")

    def __enter__(self):
        return self

    def __exit__(self, type, value, traceback):
        self.shutdown()
        return False

    def _ensure_satellite_running(self):
        if not getattr(self, 'satellites', None):
            logging.info("Starting up satellites.")
            self.satellites = MockSatelliteGroup(self._satellite_ports, self.satellite_mode)
            time.sleep(1) # wait for satellite to startup

        if not self.satellites.all_running():
            raise Exception("One or more satellites failed to start.")


    def _ensure_satellite_shutdown(self):
        if getattr(self, 'satellites', None): # if there is a satellite running
            logging.info("Shutting down satellites.")
            self.satellites.terminate()
            self.satellites = None

    def shutdown(self):
        print("Controller shutdown called")
        self._ensure_satellite_shutdown() # stop satellites
        self.server.server_close() # unbind controller server from socket
        logging.info("Controller shutdown complete")

    """ Estimate how much work per second the client does. Although in practice
    this is slightly dependent on work and repeat values, it is mostly dependent on
    the sleep value. """
    def _estimate_work_per_second(self):
        work = 1000
        result = self.raw_benchmark(Command(
            trace=False,
            with_satellites=False,
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
                with_satellites=False,
                sleep=sleep_per_work * work,
                work=work,
                repeat=5000))

            if abs(result.cpu_usage - target_cpu_usage) < .005: # within 1/2 a percent
                return sleep_per_work

            # make sure sleep per work is in range [1, 1000]
            sleep_per_work = np.clip(sleep_per_work + (result.cpu_usage - target_cpu_usage) * p_constant, 1, 1000)

        return sleep_per_work


    def set_satellite_mode(self, mode):
        self._satellite_mode = mode
        self._ensure_satellite_shutdown()
        self._ensure_satellite_running()

    @property
    def satellite_mode(self):
        return self._satellite_mode

    def benchmark(self,
            trace=True,
            no_flush=False, # we typically want flush included with our measurements
            with_satellites=True,
            spans_per_second=100,
            sleep_interval=10**7,
            runtime=10):

        if spans_per_second == 0:
            raise Exception("Cannot target 0 spans per second.")

        if runtime < 1:
            raise Exception("Runtime needs to be longer than 1s.")

        if runtime > LONGEST_TEST:
            raise Exception("Runtime cannot be longer than 30s.")

        work = self._work_per_second / spans_per_second
        repeat = self._work_per_second * runtime / work

        return self.raw_benchmark(Command(
            trace=True,
            no_flush=no_flush,
            with_satellites=True,
            sleep_interval=sleep_interval,
            sleep=int(work * self._sleep_per_work),
            work=int(work),
            repeat=int(repeat)))


    def raw_benchmark(self, command):
        # save commands to server, where they will be used to control stuff
        self.server.add_command(command)
        self.server.add_command(Command.exit())

        number_commands = 2

        if command.with_satellites:
            self._ensure_satellite_running()
            self.satellites.reset_spans_received()
        else:
            self._ensure_satellite_shutdown()

        # startup test process
        with open(f'logs/{self.client_name}.log', 'a+') as logfile:
            logging.info("Starting client...")
            client_handle = subprocess.Popen(self.client_startup_args, stdout=logfile, stderr=logfile)
            logging.info("Client started.")

            while self.server.length_results() < number_commands:
                self.server.handle_request()

            # at this point, we have sent the exit command and received a response
            # wait for the client program to shutdown
            logging.info("Waiting for client to shutdown...")
            while client_handle.poll() == None:
                pass
            logging.info("Client shutdown.")

        spans_received = self.satellites.get_spans_received() if command.with_satellites else 0

        # removes results from queue
        # don't include that last result because it's from the exit command
        result = self.server.pop_results()[0]
        result.spans_received = spans_received

        return result
