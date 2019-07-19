from controller import Controller, Command

controller = Controller(['python3', 'clients/python_client.py', 'vanilla'], client_name='vanilla_python_client')


print("target spans / s, actual spans / s, dropped spans, percent spans dropped, cpu usage")

try:
    print(f'*** Satellite Disconnect Test ***')
    for sps in range(0, 2500, 100):
        for i in range(0, 5):
            result = controller.benchmark(Command(sps, trace=True, sleep=2*10**5, with_satellites=False, test_time=2))
            print(f'{sps}, {result.spans_per_second}, {result.cpu_usage}')

finally:
    controller.shutdown()
