from controller import Controller, Command
from scipy import stats
import matplotlib.pyplot as plt
import numpy as np

controller = Controller(['python3', 'clients/python_client.py', 'vanilla'], client_name='vanilla_python_client')


def compute_gradiant(target_cpu_usage, target_spans_per_second, work, sleep):

    work_learning_rate = 100000
    sleep_learning_rate = 10000000

    for i in range(0, 20):
        work_0 = int(work)
        work_1 = int(work * 1.1)

        sleep_0 = int(sleep)
        sleep_1 = int(sleep * 1.1)

        baseline = controller.benchmark(Command(
            sleep=sleep_0,
            work=work_0,
            repeat=1000,
            trace=False,
            with_satellites=False,
        ))

        sleep_test = controller.benchmark(Command(
            sleep=sleep_1,
            work=work_0,
            repeat=1000,
            trace=False,
            with_satellites=False,
        ))

        work_test = controller.benchmark(Command(
            sleep=sleep_0,
            work=work_1,
            repeat=1000,
            trace=False,
            with_satellites=False,
        ))

        # print(f'baseline: {baseline.cpu_usage}, {baseline.spans_per_second}')
        # print(f'test: {test.cpu_usage}, {test.spans_per_second}')

        cost_func = lambda x : (x.cpu_usage - target_cpu_usage) ** 2 + ((x.spans_per_second - target_spans_per_second) / target_spans_per_second) ** 2

        # partial
        cost_per_work = (cost_func(work_test) - cost_func(baseline)) / (work_1 - work_0)
        cost_per_sleep = (cost_func(sleep_test) - cost_func(baseline)) / (sleep_1 - sleep_0)

        # lets go DOWN the gradiant !
        work -= cost_per_work * work_learning_rate
        sleep -= cost_per_sleep * sleep_learning_rate

        print(f'cost_per_work: {cost_per_work}, cost per sleep: {cost_per_sleep}')
        print(f'sps: {baseline.spans_per_second}, cpu: {baseline.cpu_usage} ({sleep}, {work})')




# try:
#     for work in range(10000, 40000, 5000):
#         for i in range(0, 5):
#             result = controller.benchmark(Command(
#                 trace=True,
#                 with_satellites=True,
#                 sleep=4*10**5,
#                 work=work,
#                 repeat=1000))
#
#             print(f'{result.spans_per_second} {result.cpu_usage}')
#
# finally:
#     controller.shutdown()

try:
    print("NoOp")
    for work in range(10000, 40000, 5000):
        for i in range(0, 20):
            result = controller.benchmark(Command(
                trace=False,
                with_satellites=False,
                sleep=4*10**5,
                work=work,
                repeat=1000))

            print(f'{result.spans_per_second} {result.cpu_usage}')

    print("LS")
    for work in range(10000, 40000, 5000):
        for i in range(0, 20):
            result = controller.benchmark(Command(
                trace=True,
                with_satellites=True,
                sleep=4*10**5,
                work=work,
                repeat=1000))

            print(f'{result.spans_per_second} {result.cpu_usage}')

finally:
    controller.shutdown()
