import threading
import time
from os import path
import logging
from .utils import PROJECT_DIR, start_logging_subprocess
from .exceptions import InvalidClient, ClientTimeout
import psutil
from threading import Thread, Lock
import subprocess

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
    'js': [
        'node', path.join(PROJECT_DIR, 'clients/js_client.js'),
    ],
    'go': [
        path.join(PROJECT_DIR, 'clients/go_client'),
    ],
    'cpp': [
        path.join(PROJECT_DIR, 'clients/cpp_client'),
    ],
}

logger = logging.getLogger(__name__)


def get_client_args(command):
    return [
        '--trace', str(int(command['Trace'])),
        '--sleep', str(command['Sleep']),
        '--sleep_interval', str(command['SleepInterval']),
        '--work', str(command['Work']),
        '--repeat', str(command['Repeat']),
        '--no_flush', str(int(command['NoFlush'])),
    ]


class CommandHandle:
    def __init__(self):
        self._lock = threading.Lock()
        self._command = None

    def handle_timeout(self):
        logger.error('Timeout waiting for client to complete test. ' +
                     'Try running the test with `no_timeout = True`?')

        raise ClientTimeout()

    def run_test(self, command, process_handle):
        logger.info("execute command")

        while process_handle.poll() is None:
            pass

        logger.info("execute command finished.")

        return process_handle.get_results()


class ClientProcess(subprocess.Popen):
    # Client process is now not expected to send back anything.
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)

        # these are locked because they are accessed from two threads
        self._lock = Lock()
        self._cpu_list = []
        self._memory_list = []

        save_list_stats_thread = Thread(
            target=self._save_list_stats,
            args=(self._cpu_list, self._memory_list))
        save_list_stats_thread.daemon = True
        save_list_stats_thread.start()

        save_runtime_stats_thread = Thread(
            target=self._save_runtime_stats,
            args=())
        save_runtime_stats_thread.daemon = True
        save_runtime_stats_thread.start()

    @property
    def cpu_list(self):
        with self._lock:
            return self._cpu_list.copy()

    @property
    def memory_list(self):
        with self._lock:
            return self._memory_list.copy()

    @property
    def program_time(self):
        if hasattr(self, "_program_time"):
            with self._lock:
                return self._program_time
        raise AttributeError()

    @property
    def clock_time(self):
        if hasattr(self, "_clock_time"):
            with self._lock:
                return self._clock_time
        raise AttributeError()

    def _save_runtime_stats(self):
        # we use a separate resource monitor here because we don't want
        # threading conflicts
        resource_monitor = psutil.Process(pid=self.pid)
        resource_monitor.cpu_percent()  # throw away process CPU usage so far
        start_clock_time = time.time()

        while True:
            time.sleep(.01)

            if self.poll() is not None:
                logger.debug(
                    'Client no longer running, stopping save runtime stats.')
                return

            try:
                user, system, _, _ = resource_monitor.cpu_times()
                with self._lock:
                    self._program_time = user + system
                    self._clock_time = time.time() - start_clock_time
            # if the child process has shut down, ignore because self.poll()
            # will detect this soon
            except (psutil.AccessDenied, psutil.NoSuchProcess):
                pass

    def _save_list_stats(self, cpu_list, memory_list):
        # we use a separate resource monitor here because we don't want
        # threading conflicts
        resource_monitor = psutil.Process(pid=self.pid)
        resource_monitor.cpu_percent()  # throw away process CPU usage so far

        while True:
            time.sleep(1)

            # if the underlying process is no longer running, don't modify cpu
            # or memory
            if self.poll() is not None:
                logger.debug(
                    'Client no longer running, stopping save list stats.')
                return

            try:
                cpu_percent = resource_monitor.cpu_percent()
                memory_usage = resource_monitor.memory_info()[0]

                with self._lock:
                    cpu_list.append(cpu_percent / 100)  # convert to decimal
                    memory_list.append(memory_usage)
            # if the child process has shut down, ignore because self.poll()
            # will detect this soon
            except (psutil.AccessDenied, psutil.NoSuchProcess):
                pass

    def get_results(self):
        return Result(
            0,  # spans_sent will be set later
            self.program_time,
            self.clock_time,
            self.memory_list,
            self.cpu_list)


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
        self.command_handle = CommandHandle()
        logger.info("Started controller server.")

        self._calibrate(target_cpu_usage)

    def _calibrate(self, target_cpu_usage):
        # timeout used during client calibration
        self.command_handle.timeout = CALIBRATION_TIMEOUT

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
        pass

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
            self.command_handle.timeout = runtime * 2
        else:
            self.command_handle.timeout = None

        # make sure that satellite span counters are all zeroed
        # throws an error if the satellites aren't running
        if satellites:
            satellites.reset_spans_received()

        result = self._raw_benchmark({
            'Trace': trace,
            'Sleep': int(work * self._sleep_per_work),
            'SleepInterval': DEFAULT_SLEEP_INTERVAL,
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
            self.client_startup_args + get_client_args(command),
            client_logger,
            popen_class=ClientProcess)

        logger.info("Client test started.")
        results = self.command_handle.run_test(command, client_handle)
        logger.info("Client test stopped.")

        results.spans_sent = int(command['Repeat'])

        return results
