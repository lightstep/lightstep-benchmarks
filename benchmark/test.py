from .controller import Controller, Command, Result
from .satellite import MockSatelliteGroup as SatelliteGroup
import matplotlib.pyplot as plt
import numpy as np
import pytest
from .generated import collector_pb2 as collector
import requests
from time import time, sleep

class TestController:


    def test_benchmark_calibration(self):
        """ Tests the controller's CPU usage calibration -- make sure that it is
        accurate to 5%. """

        with Controller('python', target_cpu_usage=.7) as controller:
            result = controller.benchmark(trace=False, runtime=10)
            assert abs(result.cpu_usage - .7) < .05




class TestMockSatelliteGroup:
    def test_all_running(self):
        satellites = SatelliteGroup('typical')
        assert satellites.all_running() == True

        satellites.shutdown()
        assert satellites.all_running() == False


    def test_shutdown_start(self):
        satellites = SatelliteGroup('typical')
        satellites.shutdown()
        assert satellites.all_running() == False

        satellites.start('typical')
        assert satellites.all_running() == True

        satellites.shutdown()
        assert satellites.all_running() == False

    def test_with_statement(self):
        """ Satellites should shutdown when program exits scope. """

        with SatelliteGroup('typical') as satellites:
            assert satellites.all_running() == True

        assert satellites.all_running() == False


    def test_spans_received(self):
        with Controller('python') as controller:
            with SatelliteGroup('typical') as satellites:
                assert satellites.get_spans_received() == 0

                # TODO: replace this with a simple tracer that doesn't
                # depend on this whole controller class working
                result = controller.benchmark(
                    trace=True,
                    satellites=satellites,
                    spans_per_second=100,
                    runtime=5
                )
                assert result.spans_sent == satellites.get_spans_received()

                satellites.reset_spans_received()
                assert satellites.get_spans_received() == 0

    def test_startup_fail(self):
        """ Satellites should raise an exception if we try to start two
        instances, because they bind on the same ports. """

        with SatelliteGroup('typical') as satellites:
            with pytest.raises(Exception) as exception_info:
                new_satellites = SatelliteGroup('typical')

            assert exception_info.type == Exception

    def _make_report_request(self, number_spans):
        """ make a very simple 50 span report """

        report_request = collector.ReportRequest()
        span = collector.Span()
        span.operation_name = "isaac_op"
        for i in range(number_spans):
            report_request.spans.append(span)
        return report_request.SerializeToString()

    def test_satellite_throughput(self):
        """ Make sure that a single satellite can ingest spans at a rate of
        at least 2000 / second without dropping any. """

        SPANS_IN_REPORT_REQUEST = 100
        TEST_LENGTH = 10

        report_request = self._make_report_request(SPANS_IN_REPORT_REQUEST)

        with SatelliteGroup('typical') as satellites:
            start_time = time()
            spans_sent = 0
            while time() < start_time + TEST_LENGTH:
                res = requests.post(url='http://localhost:8360/api/v2/reports',
                                    data=report_request,
                                    headers={'Content-Type': 'application/octet-stream'})
                spans_sent += SPANS_IN_REPORT_REQUEST

            test_time = time() - start_time
            spans_received = satellites.get_spans_received()

            spans_dropped = spans_sent - spans_received
            spans_per_second = spans_sent / test_time

            assert spans_dropped == 0
            assert spans_per_second > 2000
