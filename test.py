from controller import Controller, Command, Result
from scipy import stats
import matplotlib.pyplot as plt
import numpy as np
import pytest


@pytest.fixture(scope="module")
def vanilla_controller():
    with Controller(['python3', 'clients/python_client.py', 'vanilla'],
            client_name='vanilla_python_client') as c:
        yield c

def test_controller(vanilla_controller):
    result = vanilla_controller.benchmark(Command(
        trace=True,
        with_satellites=True,
        sleep=10**5,
        work=1000,
        repeat=1000))

    assert isinstance(result, Result)
    assert result.spans_sent > 0
    assert result.dropped_spans == 0
