from .controller import Controller
from .satellite import MockSatelliteGroup as SatelliteGroup
from .exceptions import DeadSatellites
import pytest
import requests
from time import time
import logging

import lightstep
# we have to do this because the locally compiled proto will conflict
from lightstep import collector_pb2 as collector

logging.basicConfig(level=logging.INFO)


class TestController:
    def test_cpu_calibration(self):
        """ Tests that the controller's CPU usage calibration is
        accurate to 4%. """

        with Controller('python', target_cpu_usage=.7) as controller:
            result = controller.benchmark(trace=False, runtime=10)
            assert abs(result.cpu_usage - .7) < .04

        with Controller('python', target_cpu_usage=.8) as controller:
            result = controller.benchmark(trace=False, runtime=10)
            assert abs(result.cpu_usage - .8) < .04

    def test_runtime_calibration(self):
        """ Tests that the controller's runtime calibration is accurate to
        20%. """

        RUNTIME = 10

        with Controller('python') as controller:
            result = controller.benchmark(trace=False, runtime=RUNTIME)
            runtime_error = abs((result.clock_time - RUNTIME) / RUNTIME)

            assert runtime_error < .2

    def test_benchmark_with_satellite(self):
        """ Test to make sure that we read dropped spans from the satellite and
        update accordingly. """
        with Controller('python') as controller:
            with SatelliteGroup('typical') as satellites:
                result = controller.benchmark(
                    trace=True,
                    spans_per_second=100,
                    satellites=satellites,
                    runtime=5
                )

                # make sure that controller calls get_spans_received on
                # satellite and updates spans_received field
                assert result.spans_received == result.spans_sent

                # make sure that controller resets spans_received after it has
                # read from the satellites
                assert satellites.get_spans_received() == 0

    def test_benchmark_no_satellite(self):
        with Controller('python') as controller:
            result = controller.benchmark(
                trace=False,
                spans_per_second=100,
                runtime=5)

            assert result.spans_received == 0
            assert result.dropped_spans == result.spans_sent

            # check result.cpu_usage
            assert isinstance(result.cpu_usage, float)
            assert result.cpu_usage >= 0 and result.cpu_usage <= 1

            # check result.cpu_list
            assert isinstance(result.cpu_list, list)
            for item in result.cpu_list:
                assert isinstance(item, float)
                assert item >= 0 and item <= 1

            # check result.memory_list
            assert isinstance(result.memory_list, list)
            for item in result.memory_list:
                assert isinstance(item, int)

            # check result.program_time and result.clock_time
            assert isinstance(result.clock_time, float)
            assert isinstance(result.program_time, float)
            assert result.clock_time > result.program_time

            # check result.spans_per_second
            assert result.spans_per_second > 0

    def test_raw_benchmark(self):
        """ Make sure that raw_benchmark sends the correct number of spans.
        """

        with Controller('python') as controller:
            result = controller._raw_benchmark({
                'Trace': False,
                'Sleep': 25,
                'SleepInterval': 10**8,
                'Work': 100,
                'Repeat': 100,
                'NoFlush': False,
                'Exit': False
            })

            assert result.spans_sent == 100


class TestMockSatelliteGroup:
    def test_all_running(self):
        satellites = SatelliteGroup('typical')
        assert satellites.all_running() is True

        satellites.shutdown()
        assert satellites.all_running() is False

    def test_shutdown_start(self):
        satellites = SatelliteGroup('typical')
        satellites.shutdown()
        assert satellites.all_running() is False

        satellites.start('typical')
        assert satellites.all_running() is True

        satellites.shutdown()
        assert satellites.all_running() is False

    def test_with_statement(self):
        """ Satellites should shutdown when program exits scope. """

        with SatelliteGroup('typical') as satellites:
            assert satellites.all_running() is True

        assert satellites.all_running() is False

    def test_spans_received(self):
        with SatelliteGroup('typical') as satellites:
            assert satellites.get_spans_received() == 0

            # send 1 span
            tracer = lightstep.Tracer(
                component_name='isaac_service',
                collector_port=8360,
                collector_host='localhost',
                collector_encryption='none',
                use_http=True,
                access_token='test'
            )
            with tracer.start_active_span('TestSpan'):
                pass
            tracer.flush()

            assert satellites.get_spans_received() == 1

    def test_startup_fail(self):
        """ Satellites should raise an exception if we try to start two
        instances, because they bind on the same ports. """

        with SatelliteGroup('typical'):
            with pytest.raises(DeadSatellites) as exception_info:
                SatelliteGroup('typical')

            assert exception_info.type == DeadSatellites

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
                requests.post(
                    url='http://localhost:8360/api/v2/reports',
                    data=report_request,
                    headers={'Content-Type': 'application/octet-stream'})
                spans_sent += SPANS_IN_REPORT_REQUEST

            test_time = time() - start_time
            spans_received = satellites.get_spans_received()

            spans_dropped = spans_sent - spans_received
            spans_per_second = spans_sent / test_time

            assert spans_dropped == 0
            assert spans_per_second > 2000
