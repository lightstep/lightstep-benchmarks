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

    sps_list = []
    cpu_list = []
    dropped_list = []

    with SatelliteGroup('typical') as satellites:
        with Controller(args.client) as controller:
            for sps in list(range(100, 1600, 100)) + [3000, 5000]:
                result = controller.benchmark(
                    trace=True,
                    satellites=satellites,
                    spans_per_second=sps,
                    runtime=10,
                )
                
                print(result)

                dropped_list.append(result.dropped_spans / result.spans_sent)
                cpu_list.append(result.cpu_usage)
                sps_list.append(result.spans_per_second)

    fig, ax = plt.subplots()
    ax.plot(sps_list, dropped_list)
    plt.title(f'{controller.client_name.title()} Spans Dropped')
    ax.set(xlabel="spans / s", ylabel="percent spans dropped")
    fig.savefig(f'graphs/{controller.client_name}_sps_vs_dropped.png')
