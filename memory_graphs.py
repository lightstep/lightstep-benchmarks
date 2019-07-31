import matplotlib.pyplot as plt
from controller import Controller
from satellite import SatelliteGroup
import numpy as np
import argparse
from os import path

if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('client', help='Name of the client to use in these tests.')
    args = parser.parse_args()

    fig, ax = plt.subplots()

    with SatelliteGroup('typical') as satellites:
        with Controller(args.client) as controller:

            for sps in [100, 500, 1000, 2000]:
                result = controller.benchmark(
                    trace=True,
                    spans_per_second=sps,
                    runtime=50,
                )

                print(result)

                runtime_list = list(range(1, len(result.memory_list) + 1))
                memory_list = [m * 2**-20 for m in result.memory_list]

                ax.plot(runtime_list, memory_list, label=f'{sps} spans / sec')

    ax.set(xlabel="runtime (s)", ylabel="memory use (MB)")
    ax.set_title(f'{controller.client_name.title()} Memory Use Over Time')
    ax.legend()
    fig.savefig(f'graphs/{controller.client_name}_runtime_vs_memory.png')
