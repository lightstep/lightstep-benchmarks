import matplotlib.pyplot as plt
from controller import Controller
import numpy as np
import argparse
from os import path
from satellite import SatelliteGroup
from threading import Timer
import logging
import time


TRIAL_LENGTH = 50
CONNECT_TIME = 30

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='Produce graphs of what happens when satellites disconnect / reconnect.')
    parser.add_argument('client', help='Name of the client to use in these tests.')
    args = parser.parse_args()

    with Controller(args.client) as controller:
        # two stacked plots in one figure
        fig, ax = plt.subplots()
        ax.set_title(f'{controller.client_name.title()} Satellite Reconnect')
        ax.set(xlabel="Time (s)", ylabel="Memory (MB)")

        # satellites = SatelliteGroup('typical')
        # satellites.shutdown()
        #
        # def satellite_action():
        #     satellites.start('typical')
        #     print("starting up satellites")
        #
        # # satellites shutdown in the middle of the test
        # shutdown_timer = Timer(CONNECT_TIME, satellite_action)
        # shutdown_timer.start()

        result = controller.benchmark(
            trace=False,
            spans_per_second=300,
            runtime=TRIAL_LENGTH,
            no_flush=True
        )

        time_list = list(range(1, len(result.memory_list) + 1))
        mem_list = [m * 2**-20 for m in result.memory_list]
        ax.plot(time_list, mem_list)

        print("[" + ",".join([str(m) for m in mem_list]) + "]")
        print("[" + ",".join([str(t) for t in time_list]) + "]")

        fig.savefig(f'graphs/{controller.client_name}_satellite_connect.png')

        # satellites.shutdown()
