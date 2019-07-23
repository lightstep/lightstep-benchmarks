import matplotlib.pyplot as plt
from controller import Controller
import numpy as np
import argparse
from os import path


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='Produce graphs of tests results.')
    parser.add_argument('dir', help='Directory to save graphs to.')
    args = parser.parse_args()

    with Controller(['python3', 'clients/python_client.py', 'vanilla'],
            client_name='vanilla_python_client',
            target_cpu_usage=.7) as controller:

        sps_list = []
        dropped_list = []

        for sps in range(100, 1500, 100):
            result = controller.benchmark(
                trace=True,
                with_satellites=True,
                spans_per_second=sps,
                runtime=5)

            print(result)

            dropped_list.append(result.dropped_spans / result.spans_sent)
            sps_list.append(result.spans_per_second)

        plt.plot(sps_list, dropped_list, label='vanilla python tracer')
        plt.xlabel("Spans Per Second")
        plt.ylabel("Percent Spans Dropped")
        plt.title("Spans Per Second vs. Spans Dropped")
        plt.savefig(path.join(args.dir, 'sps_vs_dropped.png'))

print("Done making graphs.")
