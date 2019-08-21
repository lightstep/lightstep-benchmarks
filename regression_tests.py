import numpy as np
import pytest
from benchmark.controller import Controller
from benchmark.satellite import MockSatelliteGroup as SatelliteGroup


@pytest.fixture(scope='module')
def satellites():
    with SatelliteGroup('typical') as satellites:
        yield satellites


def test_memory(client_name, satellites):
    """ Tracers should not have memory leaks. Make sure that running the tracer
    for 100s doesn't use more than twice as much memory as running it for
    5s. """

    with Controller(client_name) as controller:
        result_100s = controller.benchmark(
            trace=True,
            spans_per_second=500,
            runtime=100,
            satellites=satellites)

        result_5s = controller.benchmark(
            trace=True,
            spans_per_second=500,
            runtime=5,
            satellites=satellites)

    assert(result_100s.memory / result_5s.memory < 2)


def test_dropped_spans(client_name, satellites):
    """ No tracer should drop spans if we're only sending 300 / s. """

    with Controller(client_name) as controller:
        sps_100 = controller.benchmark(
            trace=True,
            spans_per_second=100,
            runtime=10,
            satellites=satellites)

        sps_300 = controller.benchmark(
            trace=True,
            spans_per_second=300,
            runtime=10,
            satellites=satellites)

    assert(sps_100.dropped_spans == 0)
    assert(sps_300.dropped_spans == 0)


def test_cpu(client_name, satellites):
    """ Traced ciode shouldn't consume significatly more CPU than untraced
    code. Ensure that traced code sending 500 spans / second doesn't increase
    CPU usage by more than 10%. """

    TRIALS = 5
    cpu_traced = []
    cpu_untraced = []
    with Controller(client_name) as controller:
        for i in range(TRIALS):
            result_untraced = controller.benchmark(
                trace=False,
                spans_per_second=500,
                runtime=10)
            cpu_untraced.append(result_untraced.cpu_usage * 100)

            result_traced = controller.benchmark(
                trace=True,
                spans_per_second=500,
                runtime=10,
                satellites=satellites)
            cpu_traced.append(result_traced.cpu_usage * 100)

    assert(abs(np.mean(cpu_traced) - np.mean(cpu_untraced)) < 10)


def test_max_throughput(client_name, satellites):
    """ Ensure that we can send 3000 spans / second before LightStep tracer
    uses 10% CPU. """

    SPS_INCREMENT = 1000

    with Controller(client_name) as controller:
        target_sps = SPS_INCREMENT

        while True:
            result = controller.benchmark(
                trace=True,
                spans_per_second=target_sps,
                runtime=5,
                satellites=satellites)

            if result.cpu_usage > .8:
                break

            if target_sps > 3000:
                break

            target_sps += SPS_INCREMENT

        assert result.spans_per_second > 3000
        assert result.dropped_spans == 0
