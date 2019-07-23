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
    result = vanilla_controller.benchmark(Command(
        trace=True,
        with_satellites=True,
        sleep_per_work=25,
        work=1000,
        repeat=1000))

    assert isinstance(result, Result)
    assert result.spans_sent > 0
    assert result.dropped_spans == 0

def test_cpu_usage(vanilla_controller):
    # find work per sleep which leads to 70% CPU usage
    sleep_per_work = calibrate_for_cpu(vanilla_controller, .7)

    # find work which leads to these sps rates
    work_for_100_sps = calibrate_for_sps(vanilla_controller, 100, sleep_per_work)
    work_for_1000_sps = calibrate_for_sps(vanilla_controller, 1000, sleep_per_work)

    result_100_sps = vanilla_controller.benchmark(Command(
        trace=True,
        with_satellites=True,
        sleep_per_work=sleep_per_work,
        work=work_for_100_sps,
        repeat=1000))

    print(f'at target 100 spans / sec (actually {result_100_sps.spans_per_second}) {result_100_sps.cpu_usage} cpu usage')

    result_1000_sps = vanilla_controller.benchmark(Command(
        trace=True,
        with_satellites=True,
        sleep_per_work=sleep_per_work,
        work=work_for_1000_sps,
        repeat=10000))

    print(f'at target 1000 spans / sec (actually {result_1000_sps.spans_per_second}) {result_1000_sps.cpu_usage} cpu usage')

    # the CPU difference between high span output and normal span output should
    # be less than 2% CPU
    assert result_1000_sps.cpu_usage - result_100_sps.cpu_usage < .02

## HELPER FUNCTIONS ##


""" Finds sleep per work which leads to target CPU usage. """
def calibrate_for_cpu(controller, target_cpu_usage):
    sleep_per_work = 25
    p_constant = 10

    while True:
        result = controller.benchmark(Command(
            trace=False,
            with_satellites=False,
            sleep_per_work=sleep_per_work,
            work=1000,
            repeat=5000))


        print(f'sleep per work {sleep_per_work} created {result.cpu_usage} cpu usage')

        if abs(result.cpu_usage - target_cpu_usage) < .005: # within 1/2 a percent
            return sleep_per_work

        # make sure sleep per work is in range [1, 1000]
        sleep_per_work = np.clip(sleep_per_work + (result.cpu_usage - target_cpu_usage) * p_constant, 1, 1000)

""" Finds work which leads to target spans per second, given sleep per work. """
def calibrate_for_sps(controller, target_spans_per_second, sleep_per_work):
    assert target_spans_per_second != 0

    work = 2000

    result_2000_work = controller.benchmark(Command(
        trace=False,
        with_satellites=False,
        sleep_per_work=sleep_per_work,
        work=work,
        repeat=1000)) # don't need many repeats because we aren't actually sending any

    return int(work * (result_2000_work.spans_per_second / target_spans_per_second))
