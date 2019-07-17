import psutil
import numpy as np
import time

class WorkWrapper:
    def startup(self):
        pass

    def sleep(self):
        pass

    def work(self):
        pass

    def shutdown(self):
        pass


def find_cpu_usage(work_wrapper_class, work_per_sleep, runs=100, *args, **kwargs):
    cpu_usage = []
    proc = psutil.Process()
    work_wrapper = work_wrapper_class()


    work_wrapper.startup(*args, **kwargs)

    # clears the CPU usage cache so that everything we read is a result of
    # the rig
    proc.cpu_percent()

    start_time = time.time()

    for i in range(runs):
        for i in range(work_per_sleep):
            work_wrapper.work()
        work_wrapper.sleep()

        # get CPU measurement
        cpu_usage.append(proc.cpu_percent())

    test_time = time.time() - start_time

    work_wrapper_return = work_wrapper.shutdown()

    mean = np.mean(cpu_usage)
    stderr = np.std(cpu_usage) / np.sqrt(len(cpu_usage))

    # TODO: might be nice to return some sort of fun object here...
    return (mean, stderr, test_time, work_wrapper_return)

def find_work_per_sleep(work_wrapper_class, target_cpu_usage, max_runs=500, error=.5, *args, **kwargs):
    current_cpu_usage = 0
    work_per_sleep = 10 # fewer is safer (because it's quicker) to start

    while True:
        # the closer we get to target CPU usage, the more runs we do (this helps us zero in)
        runs = int(max_runs - (max_runs / 100) * abs(current_cpu_usage - target_cpu_usage))

        current_cpu_usage, stderr, _, _ = \
            find_cpu_usage(work_wrapper_class, int(work_per_sleep), runs=runs, *args, **kwargs)
        print(f'work / sleep {int(work_per_sleep)} --> {current_cpu_usage:.1f} (+/-{stderr:.2f}, n={runs})')

        # use P control to adjust work_per_sleep based on disparity between
        # current and actual cpu usage
        work_per_sleep += (target_cpu_usage - current_cpu_usage) / 2

        # if we are within
        if abs(current_cpu_usage - target_cpu_usage) < error:
            break

    return int(work_per_sleep)
