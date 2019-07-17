import psutil
import numpy as np

class ProfilingRig:
    def __init__(self):
        self.spans_written = 0

    def startup(self):
        pass

    def sleep(self):
        pass

    def work(self):
        pass

    def shutdown(self):
        pass


def find_cpu_usage(profiling_rig_class, work_per_sleep, runs=100, *args, **kwargs):
    cpu_usage = []
    proc = psutil.Process()
    rig = profiling_rig_class()


    rig.startup(*args, **kwargs)

    # clears the CPU usage cache so that everything we read is a result of
    # the rig
    proc.cpu_percent()

    for i in range(runs):
        for i in range(work_per_sleep):
            rig.work()
        rig.sleep()

        # get CPU measurement
        cpu_usage.append(proc.cpu_percent())

    rig.shutdown()

    mean = np.mean(cpu_usage)
    stderr = np.std(cpu_usage) / np.sqrt(len(cpu_usage))

    return (mean, stderr)

def find_work_per_sleep(profiling_rig_class, target_cpu_usage, runs=100, *args, **kwargs):
    current_cpu_usage = 0
    work_per_sleep = 10 # fewer is safer (because it's quicker) to start

    while True:
        current_cpu_usage, stderr = find_cpu_usage(profiling_rig_class, int(work_per_sleep), runs=runs)
        print(work_per_sleep, " --> ", current_cpu_usage, "(", + stderr, ")")
        work_per_sleep += (target_cpu_usage - current_cpu_usage) / 5

        if abs(current_cpu_usage - target_cpu_usage) < .1:
            break

    return int(work_per_sleep)
