from controller import Controller, Command, Result
import matplotlib.pyplot as plt
import numpy as np
import pytest


## VANILLA TESTS ##

@pytest.fixture(scope="module")
def vanilla_client():
    with Controller(['python3', 'clients/python_client.py', '8360', 'vanilla'],
            client_name='vanilla_python_client') as c:
        yield c

def test_vanilla_client(vanilla_client):
    result = vanilla_controller.benchmark(
        trace=True,
        with_satellites=True,
        spans_per_second=100,
        runtime=5)

    assert isinstance(result, Result)
    assert result.spans_sent > 0
    assert result.dropped_spans == 0

def test_vanilla_cpu_usage(vanilla_client):
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

    # the CPU difference between high span output and normal span output should
    # be less than 2% CPU
    assert result_1000_sps.cpu_usage - result_100_sps.cpu_usage < .02

## CPP TESTS ##

@pytest.fixture(scope="module")
def cpp_client():
    with Controller(['python3', 'clients/python_client.py', '8360', 'cpp'],
            client_name='cpp_python_client',
            num_satellites=8) as c:
        yield c

def test_vanilla_client(cpp_client):
    result = vanilla_controller.benchmark(
        trace=True,
        with_satellites=True,
        spans_per_second=100,
        runtime=5)

    assert isinstance(result, Result)
    assert result.spans_sent > 0
    assert result.dropped_spans == 0

## VANILLA TESTS WITH SIDECAR ##

@pytest.fixture(scope="module")
def sidecar_client():
    with Controller(['python3', 'clients/python_client.py', '8024', 'vanilla'],
            client_name='sidecar_python_client') as c:
        yield c

def test_sidecar_client(sidecar_client):
    result = vanilla_controller.benchmark(
        trace=True,
        with_satellites=True,
        spans_per_second=100,
        runtime=5)

    assert isinstance(result, Result)
    assert result.spans_sent > 0
    assert result.dropped_spans == 0
