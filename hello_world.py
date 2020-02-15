from benchmark.controller import Controller
from benchmark.satellite import MockSatelliteGroup

with Controller('python', target_cpu_usage=.7) as c:
    with MockSatelliteGroup('typical') as sats:
        result = c.benchmark(
            trace=True,
            satellites=sats,
            spans_per_second=100,
            runtime=10,
            no_timeout=False)

        print(f'The test had CPU for {result.program_time} seconds')
        print(f'The test took {result.clock_time} seconds to run')
        print(f'{result.spans_sent} spans were generated during the test.')
        print(f'Percent CPU used, sampled each second: {result.cpu_list}')
        print(f'Bytes memory used, sampled each second: {result.memory_list}')

        print(f'{result.spans_received} spans were received by the mock satellites.')
        print(f'{result.dropped_spans} spans were dropped.')