import profile
import time
import lightstep
import numpy as np

class TracerProfilingRig(profile.ProfilingRig):
    def startup(self, span_frequency=None):
        self.span_frequency = span_frequency;

        if self.span_frequency:
            self._setup_tracing()

    def _setup_tracing(self):
        self.last_update_time = time.time()
        self.span_period = 1.0 / span_frequency

        self.tracer = lightstep.Tracer(
            component_name='isaac_service',
            collector_port=8012,
            collector_host='localhost',
            collector_encryption='none',
            use_http=True,
            access_token='developer'
        )


    def sleep(self):
        time.sleep(.001) # 1ms

    def work(self):
        # do busywork
        i = 1.12563
        for i in range(0, 1000):
            i *= i

        if self.span_frequency and time.time() > self.last_update_time + self.span_period:
            self.last_update_time = time.time()

            with self.tracer.start_active_span('TestSpan') as scope:
                pass
            self.spans_written += 1

    def shutdown(self):
        if self.span_frequency:
            self.tracer.flush()


# # g = MockSatelliteGroup([satellite_port])
# current_usage = 0
# target_usage = 70
# work_per_sleep = 50
#
# while abs(current_usage - target_usage) > 1:
#     current_usage, stderr = profile.find_cpu_usage(TracerProfilingRig, int(work_per_sleep), span_frequency=None)
#     print("current usage:", current_usage, "target usage:", target_usage, "work_per_sleep", work_per_sleep)
#     work_per_sleep += (target_usage - current_usage) / 5

print(profile.find_work_per_sleep(TracerProfilingRig, 70, runs=1000, span_frequency=None))

# usage = []
# stderr = []
#
# for i in range(0, 10):
#     u, s = profile.find_cpu_usage(TracerProfilingRig, 5, runs=1000, span_frequency=None)
#     usage.append(u)
#     stderr.append(s)
#
# print("min stderr is: ", min(stderr))
# print("average stderr is: ", np.mean(stderr))
# print("actual std of mean is", np.std(usage))
