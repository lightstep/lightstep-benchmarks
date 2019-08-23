from http.server import BaseHTTPRequestHandler, HTTPServer
import threading
import json
import copy
from urllib.parse import urlparse, parse_qs
import time
from os import path
import logging
from .utils import PROJECT_DIR, start_logging_subprocess
from .exceptions import InvalidClient, ClientTimeout


"""
This module is used to benchmark tracers.

Config Variables
----------------
calibration_work : int
    A guideline for how many units of work the client should complete per span
    during calibration. This amount of work should produce ~100 spans / second
    when the client program is using a NoOp tracer and not sleeping.

client_args : dict mapping str to list of str
    Keys are the names of various client programs (eg. 'python'). Values are
    lists of strings which can be run on the command-line to start the client
    program.
"""

CONTROLLER_PORT = 8023
DEFAULT_SLEEP_INTERVAL = 10**8  # ns

# since calibration_work should be set to produce roughly 100 spans / second,
# this value of CALIBARATION_REPEAT should make the test last 5 seconds.
CALIBRATION_REPEAT = 500
CALIBRATION_TIMEOUT = 60


calibration_work = 200000
client_args = {
    'python': [
        'python3', path.join(PROJECT_DIR, 'clients/python_client.py'),
        'vanilla'],
    'python-cpp': [
        'python3', path.join(PROJECT_DIR, 'clients/python_client.py'),
        'cpp'],
}

logger = logging.getLogger(__name__)


def _format_query_json(query_dict):
    # Dictionaries created by urllib.parse.parse_qs looks like {key: [value]}
    # This function take dictionaries of that format and makes them normal.

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
        logger.error('Timeout waiting for client to complete test. ' +
                     'Try running the test with `no_timeout = True`?')

        raise ClientTimeout()

    def execute_command(self, command):
        # Schedules a test for the client process to run.
        with self._lock:
            assert self._command is None
            self._command = command

        while True:
            self.handle_request()

            with self._lock:
                if self._result is not None:
                    result = copy.copy(self._result)
                    self._result = None
                    return result

    def next_command(self):
        with self._lock:
            assert self._command is not None
            command = copy.copy(self._command)
            self._command = None

        return command

    def save_result(self, result):
        assert isinstance(result, Result)

        with self._lock:
            assert self._result is None
            self._result = result


class RequestHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        parsed_url = urlparse(self.path)
        self.path = parsed_url.path  # redefine path so it excludes query str
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

        logger.debug("Client result (from query string JSON): {}".format(
            self.query_json))
        result = Result.from_dict(self.query_json)
        self.server.save_result(result)

    def log_message(self, format, *args):
        return


class Result:
    """
    Holds results from a test run by the client.

    Attributes
    ----------
    spans_sent : int
        Number of spans generated during test.
    program_time : float
        CPU seconds that the test took to complete.
    clock_time : float
        Clock time that the test took to complete.
    memory_list : list of int
        Test memory use over time, in bytes.
    cpu_list : list of float
        Percent CPU use of the test over time, from 0.0 to 1.0.
    spans_received : int
        Spans received by mock satellites. If the tests was run without mock
        satellites, this is set to 0.
    memory : int
        Memory use of test just before completion.
    spans_per_second : float
        Spans sent per second.
    dropped_spans : float
        Fraction of spans which were not received by mock satellites.
    cpu_usage : float
        Average CPU usage over the entire length of the test, from 0.0 to 1.0.
    """

    def __init__(self, spans_sent, program_time, clock_time,
                 memory_list, cpu_list, spans_received=0):
        self.spans_sent = spans_sent
        self.program_time = program_time
        self.clock_time = clock_time
        self.memory_list = memory_list
        self.cpu_list = cpu_list
        self.spans_received = spans_received

    def __str__(self):
        ret = 'controller.Results object:\n'
        ret += f'\t{self.spans_per_second:.1f} spans / sec\n'
        ret += f'\t{self.cpu_usage * 100:.2f}% CPU usage\n'
        ret += f'\t{self.memory} bytes of virtual memory used at finish\n'
        if self.spans_sent > 0:
            ret += (f'\t{self.dropped_spans / self.spans_sent * 100:.1f}' +
                    f'% spans dropped (out of {self.spans_sent} sent)\n')
        ret += f'\ttook {self.clock_time:.1f}s'

        return ret

    @staticmethod
    def from_dict(result_dict, spans_received=0):
        def to_list(val):
            return val if isinstance(val, list) else [val]

        spans_sent = result_dict.get('SpansSent', 0)
        program_time = result_dict.get('ProgramTime', 0)
        clock_time = result_dict.get('ClockTime', 0)
        memory_list = \
            [int(m) for m in to_list(result_dict.get('MemoryList', []))]
        cpu_list = \
            [float(m) for m in to_list(result_dict.get('CPUList', []))]

        return Result(
            int(spans_sent),
            float(program_time),
            float(clock_time),
            memory_list,
            cpu_list,
            spans_received=int(spans_received))

    @property
    def memory(self):
        return self.memory_list[-1]

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
    """ Harness used to benchmark tracers. """

    def __init__(self, client_name, target_cpu_usage=.7):
        """
        Create a new instance of Controller.

        Parameters
        ---------
        client_name : str
            Name of the client which will be used to perform tests
            (eg. 'python', 'python-cpp'). Clients are registered in
            `client_args` global variable.
        target_cpu_usage : float
            Calibrate client program to use `target_cpu_usage` percent CPU when
            using a NoOp tracer.

        Raises
        ------
        InvalidClient
            If `client_name` is not the name of a registered client.
        """

        if client_name not in client_args:
            raise InvalidClient()

        self.client_startup_args = client_args[client_name]
        self.client_name = client_name

        # start server that will communicate with client
        self.server = CommandServer(('', CONTROLLER_PORT), RequestHandler)
        logger.info("Started controller server.")

        self._calibrate(target_cpu_usage)

    def _calibrate(self, target_cpu_usage):
        # timeout used during client calibration
        self.server.timeout = CALIBRATION_TIMEOUT

        try:
            # calibrate the amount of work the controller does so that when we
            # are using a noop tracer the CPU usage is around 70%
            self._sleep_per_work = \
                self._estimate_sleep_per_work(target_cpu_usage)

            # calculate work / second, which we use to estimate spans / second
            self._work_per_second = self._estimate_work_per_second()

        # if calibration fails, we still want to shutdown gracefully
        except Exception:
            self.shutdown()
            raise

    def __enter__(self):
        return self

    def __exit__(self, type, value, traceback):
        self.shutdown()
        return False

    def shutdown(self):
        """ Shutdown the Controller. """

        logger.info("Controller shutdown called")
        self.server.server_close()  # unbind controller server from socket
        logger.info("Controller shutdown complete")

    def _estimate_work_per_second(self):
        # Estimate how much work per second the client does. Although in
        # practice this is slightly dependent on work and repeat values, it
        # is mostly dependent on the sleep value.
        # This varies quite a bit with spans / second, which makes it a bit
        # tricky to get exactly right. Fortunately, we just have to get in the
        # ballpark

        result = self._raw_benchmark({
            'Trace': False,
            'Sleep': calibration_work * self._sleep_per_work,
            'SleepInterval': DEFAULT_SLEEP_INTERVAL,
            'Exit': False,
            'Work': calibration_work,
            'Repeat': CALIBRATION_REPEAT,
            'NoFlush': False
        })

        work_per_second = calibration_work * result.spans_per_second

        # make sure that calibration isn't too short because we want a
        # stable measurement
        if result.clock_time < 2:
            logger.warning('Calibration ran for less than 2 seconds. ' +
                           'Try adjusting `controller.calibration_work`.')

        logger.info(f'Client completes {work_per_second} units work / sec.')
        logger.debug(result)

        return calibration_work * result.spans_per_second

    def _estimate_sleep_per_work(self, target_cpu_usage, trials=3):
        # Finds sleep per work in ns which leads to target CPU usage.

        sleep_per_work = 0

        for i in range(trials):
            # first, lets check the CPU usage without no sleeping
            result = self._raw_benchmark({
                'Trace': False,
                'Sleep': sleep_per_work * calibration_work,
                'SleepInterval': DEFAULT_SLEEP_INTERVAL,
                'Exit': False,
                'Work': calibration_work,
                'Repeat': CALIBRATION_REPEAT,
                'NoFlush': False
            })

            # make sure that calibration isn't too short because we want a
            # stable measurement
            if result.clock_time < 2:
                logger.warning('Calibration ran for less than 2 seconds. ' +
                               'Try adjusting `controller.calibration_work`.')

            logger.info('clock time: {}, program time: {}'.format(
                result.clock_time,
                result.program_time))

            logger.debug(result)

            # **assuming** that the program performs the same with added sleep
            # commands calculate the additional sleep needed throughout the
            # program to hit the target CPU usage

            additional_sleep = \
                (result.program_time / target_cpu_usage) - result.clock_time
            total_work = (calibration_work * CALIBRATION_REPEAT)
            sleep_per_work += (additional_sleep / total_work) * 10**9

            logger.info(f'sleep per work is now {sleep_per_work}ns')

        logger.info(f'sleep per work {sleep_per_work} yielded' +
                    f' {result.cpu_usage * 100}% CPU usage')

        return sleep_per_work

    def benchmark(
            self,
            satellites=None,
            trace=True,
            no_flush=False,
            spans_per_second=100,
            runtime=10,
            no_timeout=False):

        """
        Run a test using the client this Controller is bound to.

        Parameters
        ----------
        satellites : satellite.MockSatelliteGroup
            Group of satellites that client program should send data to.
            If not specified, returned `Result` object won't have accurate
            `dropped_spans` or `spans_received` attributes.
        trace : bool
            False if the client should use a NoOp tracer.
        no_flush : bool
            True if the client shouldn't flush buffered spans before ending the
            test.
        spans_per_second : int
            A guideline for the rate at which the client should try to generate
            spans. Higher rates are harder to achieve.
        runtime : int
            Approximately how long the test should run.
        no_timeout : bool
            If True, the test has no maximum duration. If False, the tests will
            be stopped after `runtime` * 2 seconds.

        Returns
        -------
        controller.Result
            The result of the test.

        Raises
        ------
        ValueError
            If `spans_per_second` is set to 0.
        """

        logger.info((
            "attempting to run test with {}, trace={}, no_flush={}, " +
            "spans_per_second={}, runtime={}, no_timeout={}").format(
                'satellites' if satellites else 'no satellites',
                trace, no_flush, spans_per_second, runtime, no_timeout))

        if spans_per_second == 0:
            raise ValueError("Cannot target 0 spans per second.")

        if runtime < 1:
            logger.warn("Test `runtime` should be longer than 1 second.")

        work = self._work_per_second / spans_per_second
        repeat = self._work_per_second * runtime / work

        # set command server timeout relative to target runtime
        if not no_timeout:
            self.server.timeout = runtime * 2
        else:
            self.server.timeout = None

        # make sure that satellite span counters are all zeroed
        # throws an error if the satellites aren't running
        if satellites:
            satellites.reset_spans_received()

        result = self._raw_benchmark({
            'Trace': trace,
            'Sleep': int(work * self._sleep_per_work),
            'SleepInterval': DEFAULT_SLEEP_INTERVAL,
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

    def _raw_benchmark(self, command):
        logger.info("Starting client...")

        client_logger = logging.getLogger(
            f'{__name__}.{self.client_name}_client')
        client_handle = start_logging_subprocess(
            self.client_startup_args,
            client_logger)

        logger.info("Client started.")

        result = self.server.execute_command(command)
        self.server.execute_command({'Exit': True})

        # at this point, we have sent the exit command and received a
        # response wait for the client program to shutdown
        logger.info("Waiting for client to shutdown...")

        while client_handle.poll() is None:
            pass

        logger.info("Client shutdown.")
        return result
