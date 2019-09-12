import matplotlib.pyplot as plt
from benchmark.satellite import MockSatelliteGroup as SatelliteGroup
from benchmark.controller import Controller
from benchmark.utils import PROJECT_DIR
import numpy as np
import argparse
from os import path, makedirs

TRIALS = 20
RUNTIME = 10

GRAPHS_DIR = path.join(PROJECT_DIR, "graphs")
RAW_TRACED_FILE = path.join(GRAPHS_DIR, 'raw_data_traced.txt')
RAW_UNTRACED_FILE = path.join(GRAPHS_DIR, 'raw_data_untraced.txt')

if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument(
        'client',
        help='Name of the client to use in these tests.')
    parser.add_argument(
        '--trials',
        nargs='?',
        type=int,
        help='Number of trials to run at each span rate.')
    parser.add_argument(
        '--runtime',
        nargs='?',
        type=int,
        help='Length of each trial.')

    args = parser.parse_args()

    makedirs(GRAPHS_DIR, exist_ok=True)

    cpu_traced = []
    cpu_untraced = []
    cpu_traced_std = []
    cpu_untraced_std = []
    sps_traced = []
    sps_untraced = []

    with SatelliteGroup('typical') as satellites:
        with Controller(args.client) as controller:
            for sps in [100, 500, 1000, 2000, 3000, 4000, 5000, 7500, 10000, 20000]:
                temp_cpu_traced = []
                temp_cpu_untraced = []
                temp_sps_traced = []
                temp_sps_untraced = []

                for i in range(args.trials):
                    result = controller.benchmark(
                        trace=True,
                        spans_per_second=sps,
                        runtime=args.runtime,
                        no_timeout=True,
                        satellites=satellites,
                    )

                    print(result)
                    temp_cpu_traced.append(result.cpu_usage * 100)
                    temp_sps_traced.append(result.spans_per_second)

                    result = controller.benchmark(
                        trace=False,
                        spans_per_second=sps,
                        runtime=args.runtime,
                        no_timeout=True,
                    )
                    print(result)
                    temp_cpu_untraced.append(result.cpu_usage * 100)
                    temp_sps_untraced.append(result.spans_per_second)

                # save all raw data from tests
                with open(RAW_TRACED_FILE, 'a+') as file:
                    for i in range(len(temp_cpu_traced)):
                        file.write(
                            f'{temp_cpu_traced[i]} {temp_sps_traced[i]}\n')

                with open(RAW_UNTRACED_FILE, 'a+') as file:
                    for i in range(len(temp_cpu_untraced)):
                        file.write(
                            f'{temp_cpu_untraced[i]} {temp_sps_untraced[i]}\n')

                cpu_traced.append(np.mean(temp_cpu_traced))
                cpu_untraced.append(np.mean(temp_cpu_untraced))

                cpu_traced_std.append(np.std(temp_cpu_traced))
                cpu_untraced_std.append(np.std(temp_cpu_untraced))

                sps_traced.append(np.mean(temp_sps_traced))
                sps_untraced.append(np.mean(temp_sps_untraced))

    # draw two distinct plots
    fig, ax = plt.subplots()

    ax.errorbar(
        sps_untraced,
        cpu_untraced,
        yerr=[cpu_std / np.sqrt(args.trials) for cpu_std in cpu_untraced_std],
        label='untraced',
        color='black')

    ax.fill_between(
        sps_untraced,
        [cpu_untraced[i] - cpu_untraced_std[i]
            for i in range(len(cpu_untraced))],
        [cpu_untraced[i] + cpu_untraced_std[i]
            for i in range(len(cpu_untraced))],
        facecolor='black',
        alpha=0.5,
        label='untraced standard deviation')

    ax.errorbar(
        sps_traced,
        cpu_traced,
        yerr=[cpu_std / np.sqrt(args.trials) for cpu_std in cpu_traced_std],
        label='traced',
        color='blue')

    ax.fill_between(
        sps_traced,
        [cpu_traced[i] - cpu_traced_std[i] for i in range(len(cpu_traced))],
        [cpu_traced[i] + cpu_traced_std[i] for i in range(len(cpu_traced))],
        facecolor='blue',
        alpha=0.5,
        label='traced standard deviation')

    ax.set(xlabel="Spans per second", ylabel="Total CPU usage (percent)")
    ax.set_title(
        f'{controller.client_name.title()} Traced vs Untraced CPU Use')
    ax.legend()
    fig.savefig(path.join(
        GRAPHS_DIR, f'{controller.client_name}_sps_vs_cpu_comparison.png'))

    # compute the difference between traced and untraced CPU usage
    cpu_difference = [
        cpu_traced[i] - cpu_untraced[i] for i in range(len(cpu_traced))]

    cpu_difference_std = [(cpu_traced_std[i]**2 + cpu_traced_std[i]**2)**.5
                          for i in range(len(cpu_traced_std))]

    # draw difference plot
    fig, ax = plt.subplots()
    ax.errorbar(
        sps_traced,
        cpu_difference,
        yerr=[cpu_std / np.sqrt(args.trials)
              for cpu_std in cpu_difference_std],
        color='blue',
        label='mean & standard error')

    ax.fill_between(
        sps_traced,
        [cpu_difference[i] - cpu_difference_std[i]
            for i in range(len(cpu_difference))],
        [cpu_difference[i] + cpu_difference_std[i]
            for i in range(len(cpu_difference))],
        facecolor='blue',
        alpha=0.5,
        label='standard deviation')

    ax.set(xlabel="Spans per second", ylabel="Tracer CPU usage (percent)")
    ax.set_title(
        f'{controller.client_name.title()} CPU Use of LightStep Tracer')
    ax.legend()
    fig.savefig(path.join(
        GRAPHS_DIR, f'{controller.client_name}_sps_vs_cpu.png'))
