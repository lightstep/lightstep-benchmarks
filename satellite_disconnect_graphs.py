import matplotlib.pyplot as plt
from controller import Controller
import numpy as np
import argparse
from os import path
from satellite import SatelliteGroup
from threading import Timer
import logging
import time

TRIALS = 3
TRIAL_LENGTH = 100
DISCONNECT_TIME = 30
RECONNECT_TIME = 60

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='Produce graphs of what happens when satellites disconnect / reconnect.')
    parser.add_argument('client', help='Name of the client to use in these tests.')
    args = parser.parse_args()

    with Controller(args.client) as controller:
        # two stacked plots in one figure
        fig, (mem_ax, cpu_ax) = plt.subplots(2, 4, sharex='col', sharey='row', figsize=(20, 8), dpi=100)
        fig.suptitle(f'{controller.client_name.title()} Satellite Disconnect')

        mem_ax[0].set_title("Untraced")
        mem_ax[0].set_ylabel("Memory (MB)")
        cpu_ax[0].set(xlabel="Time (s)", ylabel="CPU %")

        # setup satellite disconnect column
        mem_ax[1].set_title("Connected")
        cpu_ax[1].set_xlabel("Time (s)")

        # setup nominal column
        mem_ax[2].set_title("Disconnect (after 10s)")
        cpu_ax[2].set_xlabel("Time (s)")

        # setup restart column
        mem_ax[3].set_title("Reconnect (after 10s)")
        cpu_ax[3].set_xlabel("Time (s)")

        for i in range(TRIALS):
            for index, trace, action in [
                    (0, False, None),
                    (1, True, None),
                    (2, True, 'disconnect'),
                    (3, True, 'reconnect')]:

                # Don't initialize using a with statement because we are going
                # to shut this down manually.
                satellites = SatelliteGroup('typical')
                logging.info("trial {} traced {} type {}".format(index, trace, action))

                def satellite_action():
                    if action == 'disconnect':
                        logging.info("shutting down")
                        satellites.shutdown()
                    if action == 'reconnect':
                        logging.info("shutting down")
                        satellites.shutdown()
                        time.sleep(RECONNECT_TIME - DISCONNECT_TIME)
                        satellites.start('typical')
                        logging.info("reconnected")

                # satellites shutdown in the middle of the test
                shutdown_timer = Timer(DISCONNECT_TIME, satellite_action)
                shutdown_timer.start()

                result = controller.benchmark(
                    trace=trace,
                    spans_per_second=5000,
                    runtime=TRIAL_LENGTH,
                )

                logging.info("benchmark completed")

                if action != 'disconnect':
                    satellites.shutdown()

                sample_time = list(range(1, len(result.cpu_list) + 1))

                cpu_ax[index].plot(sample_time, result.cpu_list)
                mem_ax[index].plot(sample_time, [m * 2**-20 for m in result.memory_list])

        fig.savefig(f'graphs/{controller.client_name}_satellite_disconnect.png')
