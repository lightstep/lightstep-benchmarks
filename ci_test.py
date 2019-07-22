from controller import Controller, Command
from scipy import stats
import matplotlib.pyplot as plt
import numpy as np

controller = Controller(['python3', 'clients/python_client.py', 'vanilla'], client_name='vanilla_python_client')

def test_controller_works():
    try:
        print("NoOp")
        for work in range(10000, 40000, 5000):
            result = controller.benchmark(Command(
                trace=False,
                with_satellites=False,
                sleep=4*10**5,
                work=work,
                repeat=1000))

            print(f'{result.spans_per_second} {result.cpu_usage}')

        assert True
    except:
        assert False
    finally:
        controller.shutdown()
