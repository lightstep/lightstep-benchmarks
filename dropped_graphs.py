import matplotlib.pyplot as plt
from benchmark.controller import Controller
from benchmark.satellite import MockSatelliteGroup as SatelliteGroup
import argparse
from os import path, makedirs
from benchmark.utils import PROJECT_DIR

if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument(
        'client',
        help='Name of the client to use in these tests.')
    args = parser.parse_args()

    # two charts stacked on top of eachother, sharing the x axis
    fig, (cpu_ax, dropped_ax) = plt.subplots(2, 1, sharex='col')

    makedirs(path.join(PROJECT_DIR, "graphs"), exist_ok=True)

    sps_list = []
    cpu_list = []
    dropped_list = []

    with SatelliteGroup('typical') as satellites:
        with Controller(args.client) as controller:
            for sps in [500, 1000, 5000, 20000, 50000, 100000]:
                result = controller.benchmark(
                    trace=True,
                    satellites=satellites,
                    spans_per_second=sps,
                    runtime=10,
                    no_timeout=True
                )

                print(result)

                dropped_list.append(result.dropped_spans / result.spans_sent)
                cpu_list.append(result.cpu_usage * 100)
                sps_list.append(result.spans_per_second)

    dropped_ax.plot(sps_list, dropped_list)
    dropped_ax.set(
        xlabel="Spans per second",
        ylabel="Percent spans dropped",
        title=f'{controller.client_name.title()} Spans Dropped')

    cpu_ax.plot(sps_list, cpu_list)
    cpu_ax.set(
        ylabel="Percent CPU usage",
        title=f'{controller.client_name.title()} CPU Usage')

    fig.savefig(path.join(
        PROJECT_DIR,
        f'graphs/{controller.client_name}_sps_vs_dropped.png'))
