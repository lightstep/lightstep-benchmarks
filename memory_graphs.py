import matplotlib.pyplot as plt
from controller import Controller
import numpy as np
import argparse
from os import path

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='Produce graphs of tests results.')
    parser.add_argument('dir', help='Directory to save graphs to.')
    args = parser.parse_args()

    for name in ['cpp', 'vanilla', 'sidecar']:
        with Controller(['python3', 'clients/python_client.py', '8360', 'cpp' if name == 'cpp' else 'vanilla'],
                client_name=f'{name}_client',
                target_cpu_usage=.7,
                num_satellites=8) as controller:

            for sps in [500, 1000, 2000, 5000, 10000]:
                result = controller.benchmark(
                    trace=True,
                    with_satellites=True,
                    spans_per_second=sps,
                    runtime=100,
                )

                print(result)

                runtime_list = list(range(1, len(result.memory_list) + 1))
                memory_list = [m * 10**-6 for m in result.memory_list]

                plt.plot(runtime_list, memory_list, label=f'{sps} spans / sec')

        plt.xlabel("runtime (s)")
        plt.ylabel("max memory usage (MB)")
        plt.title("Memory Usage Over Time")
        plt.legend()
        plt.savefig(path.join(args.dir, f'{name}_runtime_vs_memory.png'))
