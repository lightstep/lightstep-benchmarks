from controller import Controller, Command, Result
import matplotlib.pyplot as plt
import numpy as np
import pytest

@pytest.fixture(scope="module")
def vanilla_controller():
    with Controller(['python3', 'clients/python_client.py', 'vanilla'],
            client_name='vanilla_python_client') as c:
        yield c

def test_controller(vanilla_controller):
    result = vanilla_controller.benchmark(
        trace=True,
        with_satellites=True,
        spans_per_second=100,
        runtime=5)

    assert isinstance(result, Result)
    assert result.spans_sent > 0
    assert result.dropped_spans == 0

def test_cpu_usage(vanilla_controller):
    result_100_sps = vanilla_controller.benchmark(
        trace=True,
        with_satellites=True,
        spans_per_second=100,
        runtime=10)

    result_1000_sps = vanilla_controller.benchmark(
        trace=True,
        with_satellites=True,
        spans_per_second=1000,
        runtime=10)

    print(result_100_sps)
    print(result_1000_sps)

    # the CPU difference between high span output and normal span output should
    # be less than 2% CPU
    assert result_1000_sps.cpu_usage - result_100_sps.cpu_usage < .02
