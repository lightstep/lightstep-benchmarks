import matplotlib.pyplot as plt
from controller import Controller
import numpy as np
import argparse
from os import path

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='Produce graphs of tests results.')
    parser.add_argument('dir', help='Directory to save graphs to.')
    args = parser.parse_args()

    for name in ['vanilla', 'sidecar', 'cpp']:
        port = '8024' if name == 'sidecar' else '8360'
        client_type = 'cpp' if name == 'cpp' else 'vanilla'

        with Controller(['python3', 'clients/python_client.py', port, client_type],
                client_name=f'{name}_client',
                target_cpu_usage=.7,
                num_satellites=8) as controller:

            sps_list = []
            cpu_list = []
            dropped_list = []

            for sps in list(range(100, 1600, 100)) + [3000, 5000]:
                result = controller.benchmark(
                    trace=True,
                    with_satellites=True,
                    spans_per_second=sps,
                    runtime=10,
                    no_flush=False,
                )

                print(result)

                dropped_list.append(result.dropped_spans / result.spans_sent)
                cpu_list.append(result.cpu_usage)
                sps_list.append(result.spans_per_second)

            fig, ax = plt.subplots()
            ax.plot(sps_list, dropped_list)
            plt.title("Spans Dropped")
            ax.set(xlabel="spans / s", ylabel="percent spans dropped")
            fig.savefig(path.join(args.dir, f'{name}_sps_vs_dropped.png'))
