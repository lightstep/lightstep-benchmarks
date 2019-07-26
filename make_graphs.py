import matplotlib.pyplot as plt
from controller import Controller
import numpy as np
import argparse
from os import path

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='Produce graphs of tests results.')
    parser.add_argument('dir', help='Directory to save graphs to.')
    args = parser.parse_args()

    for port, name, fname in [
            ('8360', 'vanilla', 'vanilla'),
            ('8024', 'vanilla', 'sidecar'),
            ('8360', 'cpp', 'cpp')]:

        with Controller(['python3', 'clients/python_client.py', port, name],
                client_name=f'{fname}_client',
                target_cpu_usage=.7,
                num_satellites= 8 if name=='cpp' else 1) as controller:

            sps_list = []
            cpu_list = []
            dropped_list = []
            memory_list = []

            sps_values = list(range(100, 1600, 100)) + ([2000, 3000, 4000, 8000, 16000, 32000, 64000] if name=='cpp' else [])

            for sps in sps_values:
                result = controller.benchmark(
                    trace=True,
                    with_satellites=True,
                    spans_per_second=sps,
                    runtime=30,
                    no_flush=True,
                )

                print(result)

                memory_list.append(int(result.memory / 2**10)) # bytes --> kb
                dropped_list.append(result.dropped_spans / result.spans_sent)
                cpu_list.append(result.cpu_usage)
                sps_list.append(result.spans_per_second)

            fig, ax = plt.subplots()
            ax.plot(sps_list, dropped_list)
            plt.title("Spans Dropped No Flush")
            ax.set(xlabel="Spans Per Second", ylabel="Percent Spans Dropped")
            fig.savefig(path.join(args.dir, f'{fname}_sps_vs_dropped.png'))

            fig, ax = plt.subplots()
            ax.plot(sps_list, cpu_list)
            ax.set(xlabel="Spans Per Second", ylabel="Percent CPU Utilization")
            fig.savefig(path.join(args.dir, f'{fname}_sps_vs_cpu.png'))

            fig, ax = plt.subplots()
            ax.plot(sps_list, memory_list)
            ax.set(xlabel="Spans Per Second", ylabel="Kilobytes Memory")
            fig.savefig(path.join(args.dir, f'{fname}_sps_vs_memory.png'))
