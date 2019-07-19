from controller import Controller, Command

controller = Controller(['python3', 'clients/python_client.py', 'vanilla'], client_name='vanilla_python_client')


print("target spans / s, actual spans / s, dropped spans, percent spans dropped, cpu usage")

try:
    print(f'*** LightStep tracer ***')
    for sps in range(0, 1000, 100):
        for i in range(0, 5):
            result = controller.benchmark(Command(sps, trace=True, sleep=2*10**5, with_satellites=True, test_time=2))
            print(f'{sps}, {result.spans_per_second}, {result.dropped_spans}, {result.dropped_spans / result.spans_sent if result.spans_sent else 0}, {result.cpu_usage}')


    print(f'*** NoOp tracer ***')
    for sps in range(0, 1000, 50):
        for i in range(0, 10):
            result = controller.benchmark(Command(sps, trace=False, sleep=2*10**5, with_satellites=False, test_time=2))
            print(f'{sps}, {result.spans_per_second}, {result.dropped_spans}, {result.dropped_spans / result.spans_sent if result.spans_sent else 0}, {result.cpu_usage}')

finally:
    controller.shutdown()
