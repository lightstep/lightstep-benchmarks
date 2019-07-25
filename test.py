from controller import Controller, Command, Result
import matplotlib.pyplot as plt
import numpy as np
import pytest


## VANILLA TESTS ##

class TestVanilla:
    @pytest.fixture(scope='class')
    def client(self):
        with Controller(['python3', 'clients/python_client.py', '8360', 'vanilla'],
                client_name='vanilla_python_client') as c:
            yield c

    def test_send_spans(self, client):
        result = client.benchmark(
            trace=True,
            with_satellites=True,
            spans_per_second=100,
            runtime=5)

        assert isinstance(result, Result)
        assert result.spans_sent > 0
        assert result.dropped_spans == 0


## CPP TESTS ##

class TestSidecar:
    @pytest.fixture(scope='class')
    def client(self):
        with Controller(['python3', 'clients/python_client.py', '8360', 'cpp'],
                client_name='cpp_python_client',
                num_satellites=8) as c:
            yield c

    def test_send_spans(self, client):
        result = client.benchmark(
            trace=True,
            with_satellites=True,
            spans_per_second=100,
            runtime=5)

        assert isinstance(result, Result)
        assert result.spans_sent > 0
        assert result.dropped_spans == 0


## VANILLA TESTS WITH SIDECAR ##
class TestSidecar:
    @pytest.fixture(scope='class')
    def client(self):
        with Controller(['python3', 'clients/python_client.py', '8024', 'vanilla'],
                client_name='sidecar_python_client') as c:
            yield c

    def test_send_spans(self, client):
        result = client.benchmark(
            trace=True,
            with_satellites=True,
            spans_per_second=100,
            runtime=5)

        assert isinstance(result, Result)
        assert result.spans_sent > 0
        assert result.dropped_spans == 0
