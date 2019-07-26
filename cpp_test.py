import matplotlib.pyplot as plt
from controller import Controller
import numpy as np
import argparse
from os import path

if __name__ == '__main__':
    with Controller(['python3', 'clients/python_client.py', '8360', 'cpp'],
            client_name='cpp_client',
            target_cpu_usage=.7,
            num_satellites=8) as controller:

        sps_list = []
        cpu_list = []
        dropped_list = []
        memory_list = []

        for sps, flush in [(5000, False), (100000, False), (5000, True), (100000, True)]:
            result = controller.benchmark(
                trace=True,
                with_satellites=True,
                spans_per_second=sps,
                runtime=100,
                no_flush=flush,
            )

            print(result)
