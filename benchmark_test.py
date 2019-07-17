import benchmark
import time
import lightstep
import numpy as np
from satellite_wrapper import MockSatelliteGroup

class TracedWorkWrapper(benchmark.WorkWrapper):
    def startup(self, span_frequency=None, satellite_port=8012):
        self.span_frequency = span_frequency;
        self.spans_written = 0

        if self.span_frequency:
            self._setup_tracing(satellite_port)

    def _setup_tracing(self, port):
        self.last_update_time = time.time()
        self.span_period = 1.0 / self.span_frequency

        self.tracer = lightstep.Tracer(
            component_name='isaac_service',
            collector_port=port,
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
        return self.spans_written


# # g = MockSatelliteGroup([satellite_port])
# current_usage = 0
# target_usage = 70
# work_per_sleep = 50
#
# while abs(current_usage - target_usage) > 1:
#     current_usage, stderr = profile.find_cpu_usage(TracerProfilingRig, int(work_per_sleep), span_frequency=None)
#     print("current usage:", current_usage, "target usage:", target_usage, "work_per_sleep", work_per_sleep)
#     work_per_sleep += (target_usage - current_usage) / 5



# span frequency is not a kwarg of find_work_per_sleep, so it will be passed to
# TracedWorkWrapper's startup method
work_per_sleep = benchmark.find_work_per_sleep(TracedWorkWrapper, 70, max_runs=200, span_frequency=None)
print(f'settled on work per sleep: {work_per_sleep}')


mock_satellites = MockSatelliteGroup([8012])

try:
    for spans_per_second in [100, 200, 500, 1000, 2000]:
        cpu_usage, cpu_usage_stderr, test_time, spans_sent = \
            benchmark.find_cpu_usage(TracedWorkWrapper, work_per_sleep, runs=1000, span_frequency=spans_per_second, satellite_port=8012)

        if not mock_satellites.all_running():
            raise Exception("There was a problem with one or more of the satellites")

        time.sleep(1)
        spans_received = mock_satellites.get_spans_received()
        mock_satellites.reset_spans_received()

        print(f'{(spans_sent / test_time):.1f} spans / second (target {spans_per_second}): {cpu_usage:.2f}% CPU, ({spans_sent - spans_received} spans dropped)')

finally:
    print("Terminating mock satellites.")
    mock_satellites.terminate()

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
