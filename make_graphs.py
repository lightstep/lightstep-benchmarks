import matplotlib.pyplot as plt
from controller import Controller, Command
import numpy as np
import argparse
from os import path


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='Produce graphs of tests results.')
    parser.add_argument('dir', help='Directory to save graphs to.')
    args = parser.parse_args()

    with Controller(['python3', 'clients/python_client.py', 'vanilla'], client_name='vanilla_python_client') as controller:
        spans_per_second = []
        percent_dropped = []

        for work in range(15000, 45000, 5000):
            result = controller.benchmark(Command(
                trace=True,
                with_satellites=True,
                # sleep=4*10**5,
                sleep_per_work=25, # sleep 25ns per work
                work=work,
                repeat=10000))

            print(f'spans per second: {result.spans_per_second}, dropped: {result.dropped_spans}, cpu usage: {result.cpu_usage}')

            percent_dropped.append(result.dropped_spans / result.spans_sent)
            spans_per_second.append(result.spans_per_second)

        plt.plot(spans_per_second, percent_dropped, label='vanilla python tracer')
        plt.xlabel("Spans Per Second")
        plt.ylabel("Percent Spans Dropped")
        plt.title("Spans Per Second vs. Spans Dropped")
        plt.savefig(path.join(args.dir, 'sps_vs_dropped.png'))

print("Done making graphs.")
