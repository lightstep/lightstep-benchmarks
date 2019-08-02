import matplotlib.pyplot as plt
from controller import Controller
import numpy as np
import argparse
from os import path
from satellite import SatelliteGroup
from threading import Timer
import logging
import time


TRIAL_LENGTH = 20
TRIALS = 50

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='Produce graphs of what happens when satellites disconnect / reconnect.')
    parser.add_argument('client', help='Name of the client to use in these tests.')
    args = parser.parse_args()

    with SatelliteGroup('typical') as satellites:
        with Controller(args.client) as controller:
            for sps in [50, 100, 200, 300, 400, 500, 800, 1200]:
                print("** untraced **")
                for i in range(TRIALS):
                    result = controller.benchmark(
                        trace=False,
                        spans_per_second=sps,
                        runtime=TRIAL_LENGTH,
                        no_flush=True)
                    print(result.spans_sent, result.clock_time, result.program_time)

                print("** traced **")
                for i in range(TRIALS):
                    result = controller.benchmark(
                        trace=True,
                        spans_per_second=sps,
                        runtime=TRIAL_LENGTH,
                        no_flush=True,
                        satellites=satellites)
                    print(result.spans_sent, result.clock_time, result.program_time, result.dropped_spans)
