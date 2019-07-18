import subprocess
import time
import requests

class MockSatelliteHandler:
    def __init__(self, port):
        self.logfile = open('logs/temp/mock_satellite_' + str(port) + '.log', 'w+')
        self.port = port

        # we will subtract this number from how many received spans satellites report
        # this will give us the ability to reset spans_received without even communicating
        # with satellites
        self._spans_received_baseline = 0

        self._handler = subprocess.Popen(
            ["python3", "mock_satellite.py", str(port)],
            stdout=self.logfile, stderr=self.logfile)

    def is_running(self):
        return self._handler.poll() == None

    def get_spans_received(self):
        # before trying to communicate with the mock, check if its running
        if not self.is_running():
            raise Exception("Mock satellite not running.")

        host = "http://localhost:" + str(self.port)
        res = requests.get(host + "/spans_received")

        if res.status_code != 200:
            raise Exception("Bad status code -- not able to GET /spans_received from " + host)

        try:
            return int(res.text) - self._spans_received_baseline
        except ValueError:
            raise Exception("Bad response -- expected an integer from " + host)

    def reset_spans_received(self):
        self._spans_received_baseline += self.get_spans_received()

    """ Shutdown this satellite and return its logs in string format. """
    def terminate(self):
        # cross-platform way to terminate a program
        # on Windows calls TerminateProcess, on Posix sends SIGTERM
        self._handler.terminate()

        # wait for an exit code
        while self._handler.poll() == None:
            pass

        # read & close the logfile
        logs = self.logfile.read()
        self.logfile.close()
        return logs


class MockSatelliteGroup:
    def __init__(self, ports):
        self._satellites = \
            [MockSatelliteHandler(port) for port in ports]

    def get_spans_received(self):
        return sum([s.get_spans_received() for s in self._satellites])

    def all_running(self):
        for s in self._satellites:
            if not s.is_running():
                return False
        return True

    def reset_spans_received(self):
        for s in self._satellites:
            s.reset_spans_received()

    """ Shutdown all satellites and save their logs into a single file """
    def terminate(self):
        with open('logs/mock_satellites.log', 'w+') as logfile:
            for s in self._satellites:
                logs = s.terminate()
                logfile.write(logs)
