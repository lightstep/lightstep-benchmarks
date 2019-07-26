import matplotlib.pyplot as plt
from controller import Controller
import numpy as np
import argparse
from os import path

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='Produce graphs of tests results.')
    parser.add_argument('dir', help='Directory to save graphs to.')
    args = parser.parse_args()

    with Controller(['python3', 'clients/python_client.py', '8360', 'cpp'],
            client_name='cpp_client',
            target_cpu_usage=.7,
            num_satellites=8) as controller:

        for sps in range[500, 1000, 2000, 5000, 10000]:
            runtime_list = []
            memory_list = []

            for runtime in range(10, 100, 10):
                result = controller.benchmark(
                    trace=True,
                    with_satellites=True,
                    spans_per_second=sps,
                    runtime=runtime,
                )

                print(result)

                runtime_list.append(result.clock_time)
                memory_list.append(result.memory * 10**-6)

            plt.plot(runtime_list, memory_list, label=f'{sps} sps')


    plt.xlabel("runtime (s)")
    plt.ylabel("max memory usage (MB)")
    plt.title("Tracer Memory Use @ 5000 Spans / Sec")
    plt.legend()
    plt.savefig(path.join(args.dir, f'cpp_runtime_vs_memory.png'))
