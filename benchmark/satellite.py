import time
import requests
from os import path
from .utils import BENCHMARK_DIR, start_logging_subprocess
import logging
from .exceptions import SatelliteBadResponse, DeadSatellites

DEFAULT_PORTS = list(range(8360, 8368))

logger = logging.getLogger(__name__)

BANDWIDTH_LIMIT_KB_PER_SEC = 50*1024


class MockSatelliteHandler:
    def __init__(self, port, mode):
        self.port = port

        # we will subtract this number from how many received spans satellites
        # report this will give us the ability to reset spans_received without
        # even communicating with satellites
        self._spans_received_baseline = 0

        mock_satellite_path = path.join(BENCHMARK_DIR, 'mock_satellite.py')
        mock_satellite_logger = logging.getLogger(f'{__name__}.{port}')

        self._handler = start_logging_subprocess(
            [
                # "trickle",
                # "-s",
                # "-u", str(BANDWIDTH_LIMIT_KB_PER_SEC),
                # "-d", str(BANDWIDTH_LIMIT_KB_PER_SEC),
                "python3", mock_satellite_path, str(port), mode
            ],
            mock_satellite_logger)

    def is_running(self):
        return self._handler.poll() is None

    def get_spans_received(self):
        host = "http://localhost:" + str(self.port)
        res = requests.get(host + "/spans_received")

        if res.status_code != 200:
            raise SatelliteBadResponse("Error getting /spans_received.")

        try:
            spans_received = int(res.text) - self._spans_received_baseline
            return spans_received
        except ValueError:
            raise SatelliteBadResponse("Satellite didn't sent an int.")

    def reset_spans_received(self):
        self._spans_received_baseline += self.get_spans_received()

    def terminate(self):
        # cross-platform way to terminate a program
        # on Windows calls TerminateProcess, on Posix sends SIGTERM
        self._handler.terminate()

        # wait for an exit code
        while self._handler.poll() is None:
            pass


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
            Ports the mock satellites should listen on. A mock satellite will
            be started for each specified port.

        Raises
        ------
        DeadSatellites
            If one or more of the satellites died during startup.
        """

        self._ports = ports
        self._satellites = \
            [MockSatelliteHandler(port, mode) for port in ports]

        time.sleep(1)

        if not self.all_running():
            raise DeadSatellites()

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
        None
            If the satellite group has been shutdown.

        Raises
        ------
        DeadSatellites
            If one or more of the mock satellites have died unexpctedly.
        SatelliteBadResponse
            If one or more of the mock satellites sent a bad response.
        """

        # before trying to communicate with the mock, check if its running
        if not self._satellites or not self.all_running():
            raise DeadSatellites("One or more satellites is not running.")

        received = sum([s.get_spans_received() for s in self._satellites])
        logger.info(f'All satellites have {received} spans.')
        return received

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
        received to 0. Does nothing if the satellite group has been shutdown.\

        Raises
        ------
        SatelliteBadResponse
            If we were unable to reset the number of spans received.
        """

        if not self._satellites:
            logger.warn(
                "Cannot reset spans received since satellites are shutdown.")
            return

        logger.info("Resetting spans received.")
        for s in self._satellites:
            s.reset_spans_received()

    def start(self, mode, ports=DEFAULT_PORTS):
        """ Restarts the group of mock satellites. Should only be called if the
        group is currently shutdown.

        Parameters
        ----------
        mode : str
            Mode deteremines the response characteristics, like timing, of the
            mock satellites. Can be 'typical', 'slow_succeed', or 'slow_fail'.
        ports : list of int
            Ports the mock satellites should listen on. A mock satellite will
            be started for each specified port.
        """

        if self._satellites:
            logger.warn(
                "Cannot startup satellites because they are already running.")
            return

        logger.info("Starting up mock satellite group.")
        self.__init__(mode, ports=ports)

    def shutdown(self):
        """ Shutdown all satellites. Should only be called if the satellite
        group is currently running.
        """

        if not self._satellites:
            logger.warn(
                "Cannot shutdown satellites since they are already shutdown.")
            return

        logger.info("Shutting down mock satellite group.")
        for s in self._satellites:
            s.terminate()

        self._satellites = None
