from controller import Controller, Command, Result
from satellite import SatelliteGroup
import matplotlib.pyplot as plt
import numpy as np
import pytest
import time


class TestSatelliteGroup:
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

    # TODO: write satelite mode tests
    def test_modes(self):
        pass

class TestController:
    def test_
