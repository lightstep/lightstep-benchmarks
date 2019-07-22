import subprocess
from http.server import BaseHTTPRequestHandler, HTTPServer
import threading
import json
import copy
from urllib.parse import urlparse, parse_qs
from satellite_wrapper import MockSatelliteGroup
import time
import os


CONTROLLER_PORT = 8023
SATELLITE_PORT = 8012
VALID_COMMAND_KEYS = ['Trace', 'Sleep', 'SpansPerSecond', 'NoFlush', 'TestTime', 'Exit', 'NumLogs', 'BytesPerLog', 'SleepInterval']

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
            print("Client requested a command, but no more commands were available.")
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
            # spans_per_second,
            trace=True,
            sleep_per_work=100,
            sleep_interval=10**7,
            # test_time=5,
            work=1000,
            repeat=1000,
            with_satellites=True,
            exit=False):
        # self._spans_per_second = spans_per_second
        self._trace = trace
        self._sleep = sleep_per_work * work
        self._sleep_interval = sleep_interval
        # self._test_time = test_time
        self._with_satellites = with_satellites
        self._exit = exit
        self._work = work
        self._repeat = repeat

    @staticmethod
    def exit():
        return Command(0, exit=True)

    @property
    def with_satellites(self):
        return self._with_satellites

    def to_dict(self):
        return {
            # 'SpansPerSecond': self._spans_per_second,
            'Trace': self._trace,
            'Sleep': self._sleep,
            'SleepInterval': self._sleep_interval,
            # 'TestTime': self._test_time,
            'Exit': self._exit,
            'Work': self._work,
            'Repeat': self._repeat,
        }


''' allows us to set spans_received even after initializing ...'''
class Result:
    def __init__(self, spans_sent, program_time, clock_time, spans_received=0):
        self._spans_sent = spans_sent
        self._program_time = program_time
        self._clock_time = clock_time
        self.spans_received = spans_received

    @staticmethod
    def from_dict(result_dict, spans_received=0):
        spans_sent = result_dict.get('SpansSent', 0)
        program_time = result_dict.get('ProgramTime', 0)
        clock_time = result_dict.get('ClockTime', 0)

        return Result(int(spans_sent), float(program_time), float(clock_time), spans_received=int(spans_received))

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
    def __init__(self, client_startup_args, client_name='client'):
        # make all of the required directories
        os.makedirs("logs/temp", exist_ok=True)

        self.client_startup_args = client_startup_args
        self.client_name = client_name

        # start server that will communicate with client
        self.server = CommandServer(('', CONTROLLER_PORT), RequestHandler)

        # server.handle_request() will fail after 30 seconds since at that point
        # the client is probably dead
        self.server.timeout = 30

    def __enter__(self):
        return self

    def __exit__(self, type, value, traceback):
        self.shutdown()
        return False

    def _ensure_satellite_running(self):
        if not getattr(self, 'satellites', None):
            print("Starting up satellites.")
            self.satellites = MockSatelliteGroup([SATELLITE_PORT])
            time.sleep(1) # wait for satellite to startup

    def _ensure_satellite_shutdown(self):
        if getattr(self, 'satellites', None): # if there is a satellite running
            print("Shutting down satellites.")
            self.satellites.terminate()
            self.satellites = None

    def shutdown(self):
        print("Controller shutdown called")
        self._ensure_satellite_shutdown()

    def benchmark(self, command):
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
        with open(f'logs/{self.client_name}.log', 'w+') as logfile:
            client_handle = subprocess.Popen(self.client_startup_args, stdout=logfile, stderr=logfile)

            while self.server.length_results() < number_commands:
                self.server.handle_request()

            # at this point, we have sent the exit command and received a response
            # wait for the client program to shutdown
            while client_handle.poll() == None:
                pass

        spans_received = self.satellites.get_spans_received() if command.with_satellites else 0

        # removes results from queue
        # don't include that last result because it's from the exit command
        result = self.server.pop_results()[0]
        result.spans_received = spans_received

        return result
