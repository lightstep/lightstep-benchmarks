import subprocess
import time
import requests

class MockSatelliteHandler:
    def __init__(self, port):
        self.logfile = open('logs/temp/mock_satellite_' + str(port) + '.log', 'w+')
        self.errfile = open('logs/temp/mock_satellite_err_' + str(port) + '.log', 'w+')
        self.port = port

        self._handler = subprocess.Popen(
            ["python3", "mock_satellite.py", str(port)],
            stdout=self.logfile, stderr=self.errfile)

    def is_running(self):
        print(self._handler.poll())
        return self._handler.poll() == None

    def get_spans_received(self):
        host = "http://localhost:" + str(self.port)
        res = requests.get(host + "/spans_received")

        if res.status_code != 200:
            raise Exception("Bad status code -- not able to GET /spans_received from " + host)

        try:
            return int(res.text)
        except ValueError:
            raise Exception("Bad response -- expected an integer from " + host)

    def terminate(self):
        # cross-platform way to terminate a program
        # on Windows calls TerminateProcess, on Posix sends SIGTERM
        self._handler.terminate()


class MockSatelliteGroup:
    def __init__(self, start_port=8010, number=8):
        self._satellites = \
            [MockSatelliteHandler(port) for port in range(start_port, start_port + number)]

    def get_spans_received(self):
        return sum([s.get_spans_received() for s in self._satellites])

    def all_running(self):
        for s in self._satellites:
            if not s.is_running():
                return False
        return True

    def _concat_error_logs(self):
        with open('logs/mock_satellite_err.log', 'w+') as errfile:
            for s in self._satellites:
                errfile.write(s.errfile.read())

    def terminate(self):
        for s in self._satellites:
            s.terminate()

        self._concat_error_logs()



g = MockSatelliteGroup()
try:
    time.sleep(10)
    print("all running?:", g.all_running())
    print("got a total of:", g.get_spans_received())
except:
    print("something went wrong!")
finally:
    g.terminate()
