import matplotlib.pyplot as plt
from benchmark.satellite import MockSatelliteGroup as SatelliteGroup
from benchmark.controller import Controller
from benchmark.utils import PROJECT_DIR
import numpy as np
import argparse
from os import path, makedirs



TRIALS = 20
RUNTIME = 10

if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('client', help='Name of the client to use in these tests.')
    parser.add_argument('--trials', nargs='?', type=int, help='Number of trials to run at each span rate.')
    parser.add_argument('--runtime', nargs='?', type=int, help='Length of each trial.')

    args = parser.parse_args()

    makedirs(path.join(PROJECT_DIR, "graphs"), exist_ok=True)

    cpu_traced = []
    cpu_untraced = []
    cpu_traced_std = []
    cpu_untraced_std = []
    sps_traced = []
    sps_untraced = []

    with SatelliteGroup('typical') as satellites:
        with Controller(args.client) as controller:
            for sps in [100, 300, 500, 800, 1000]:
                temp_cpu_traced = []
                temp_cpu_untraced = []
                temp_sps_traced = []
                temp_sps_untraced = []

                for i in range(args.trials):
                    result = controller.benchmark(
                        trace=True,
                        spans_per_second=sps,
                        runtime=args.runtime,
                    )
                    print(result)
                    temp_cpu_traced.append(result.cpu_usage * 100)
                    temp_sps_traced.append(result.spans_per_second)

                    result = controller.benchmark(
                        trace=False,
                        spans_per_second=sps,
                        runtime=args.runtime,
                    )
                    print(result)
                    temp_cpu_untraced.append(result.cpu_usage * 100)
                    temp_sps_untraced.append(result.spans_per_second)

                cpu_traced.append(np.mean(temp_cpu_traced))
                cpu_untraced.append(np.mean(temp_cpu_untraced))

                cpu_traced_std.append(np.std(temp_cpu_traced))
                cpu_untraced_std.append(np.std(temp_cpu_untraced))

                sps_traced.append(np.mean(temp_sps_traced))
                sps_untraced.append(np.mean(temp_sps_untraced))

    # compute the difference between traced and untraced CPU usage
    cpu_difference = [cpu_traced[i] - cpu_untraced[i] for i in range(len(cpu_traced))]
    cpu_difference_std = [(cpu_traced_std[i]**2 + cpu_traced_std[i]**2)**.5 for i in range(len(cpu_traced_std))]

    # draw two distinct plots
    fig, ax = plt.subplots()
    ax.errorbar(sps_traced, cpu_traced, yerr=cpu_traced_std, label='traced')
    ax.errorbar(sps_untraced, cpu_untraced, yerr=cpu_untraced_std, label='untraced')
    ax.set(xlabel="spans / second", ylabel="CPU Usage")
    ax.set_title(f'{controller.client_name.title()} Traced and Untraced CPU Use')
    ax.legend()
    fig.savefig(path.join(PROJECT_DIR, f'graphs/{controller.client_name}_sps_vs_cpu_comparison.png'))

    # draw difference ploit
    fig, ax = plt.subplots()
    ax.errorbar(sps_untraced, cpu_difference, yerr=cpu_difference_std)
    ax.set(xlabel="spans / second", ylabel="Tracer CPU Usage")
    ax.set_title(f'{controller.client_name.title()} CPU Cost of Tracer')
    fig.savefig(path.join(PROJECT_DIR, f'graphs/{controller.client_name}_sps_vs_cpu.png'))
