import subprocess
import time
import requests
import os
from os import path
from .utils import PROJECT_DIR, BENCHMARK_DIR
import logging


DEFAULT_PORTS = list(range(8360, 8368))

class MockSatelliteHandler:
    def __init__(self, port, mode):
        os.makedirs(path.join(PROJECT_DIR, "logs/temp"), exist_ok=True)
        self.logfile = open(path.join(PROJECT_DIR, f'logs/temp/mock_satellite_{str(port)}.log'), 'w+')
        self.port = port

        # we will subtract this number from how many received spans satellites report
        # this will give us the ability to reset spans_received without even communicating
        # with satellites
        self._spans_received_baseline = 0

        mock_satellite_path = path.join(BENCHMARK_DIR, 'mock_satellite.py')

        self._handler = subprocess.Popen(
            ["python3", mock_satellite_path, str(port), mode],
            stdout=self.logfile, stderr=self.logfile)

    def is_running(self):
        return self._handler.poll() == None

    def get_spans_received(self):
        host = "http://localhost:" + str(self.port)
        res = requests.get(host + "/spans_received")

        if res.status_code != 200:
            raise Exception("Bad status code -- not able to GET /spans_received from " + host)

        try:
            spans_received = int(res.text) - self._spans_received_baseline
            logging.info(f'{self.port}: reported {spans_received} spans')
            return spans_received
        except ValueError:
            raise Exception("Bad response -- expected an integer from " + host)

    def reset_spans_received(self):
        logging.info(f'{self.port}: reset spans received')
        self._spans_received_baseline += self.get_spans_received()


    def terminate(self):
        # Shutdown this satellite and return its logs in string format.

        # cross-platform way to terminate a program
        # on Windows calls TerminateProcess, on Posix sends SIGTERM
        self._handler.terminate()

        # wait for an exit code
        while self._handler.poll() == None:
            pass

        # read & close the logfile
        self.logfile.seek(0) # seek to beginning of file
        logs = self.logfile.read()
        self.logfile.close()
        return logs


class MockSatelliteGroup:
    """ A group of mock satellites. """

    def __init__(self, mode, ports=DEFAULT_PORTS):
        """ Initializes and starts a group of mock satellites.

        Parameters
        ----------
        mode : str
            Mode deteremines the response characteristics, like timing, of the
            mock satellites. Can be 'typical', 'slow_succeed', or 'slow_fail'.
        ports : list of int
            Ports the mock satellites should listen on. A mock satellite will be
            started for each specified port.

        Raises
        ------
        Exception
            If the group of mock satellites is currently running.
        """

        os.makedirs(path.join(PROJECT_DIR, "logs"), exist_ok=True)

        self._ports = ports
        self._satellites = \
            [MockSatelliteHandler(port, mode) for port in ports]

        time.sleep(1)

        if not self.all_running():
            raise Exception("Couldn't start all satellites.")

    def __enter__(self):
        return self

    def __exit__(self, type, value, traceback):
        self.shutdown()
        return False

    def get_spans_received(self):
        """ Gets the number of spans that mock satellites have received.

        Returns
        -------
        int
            The number of spans that the mock satellites have received.

        Raises
        ------
        Exception
            If one or more of the mock satellites aren't running.
            If one or more of the mock satellites sent a bad response.
        """
        # before trying to communicate with the mock, check if its running
        if not self.all_running():
            raise Exception("Can't get spans received since not all satellites are running.")

        return sum([s.get_spans_received() for s in self._satellites])

    def all_running(self):
        """ Checks if all of the mock satellites in the group are running.

        Returns
        -------
        bool
            Whether or not the satellites are running.
        """

        # if the satellites are shutdown, they aren't running
        if not self._satellites:
            return False

        for s in self._satellites:
            if not s.is_running():
                return False
        return True

    def reset_spans_received(self):
        """ Resets the number of spans that the group of mock satellites have
        received to 0.

        Raises
        ------
        Exception
            If the group of mock satellites have been shutdown.
        """

        if not self._satellites:
            raise Exception("Can't reset spans received since no satellites are running.")

        for s in self._satellites:
            s.reset_spans_received()

    def start(self, mode, ports=DEFAULT_PORTS):
        """ Restarts the group of mock satellites. Can only be called if the the
        group is currently shutdown.

        Parameters
        ----------
        mode : str
            Mode deteremines the response characteristics, like timing, of the
            mock satellites. Can be 'typical', 'slow_succeed', or 'slow_fail'.
        ports : list of int
            Ports the mock satellites should listen on. A mock satellite will be
            started for each specified port.

        Raises
        ------
        Exception
            If the group of mock satellites is currently running.
        """

        if not self._satellites:
            self.__init__(mode, ports=ports)
        else:
            raise Exception("Can't call startup since satellites are running.")

    def shutdown(self):
        """ Shutdown all satellites and saves logs to logs/mock_satellites.log

        Raises
        ----------
        Exception
            If there are no satellites running to begin with.
        """

        if not self._satellites:
            raise Exception("Can't call terminate since there are no satellites running")

        with open(path.join(PROJECT_DIR, 'logs/mock_satellites.log'), 'a+') as logfile:
            logfile.write('**********\n')
            for s in self._satellites:
                logs = s.terminate()
                logfile.write(f'*** logs from satellite {s.port} ***\n')
                logfile.write(logs)
            logfile.write('\n')

        self._satellites = None
