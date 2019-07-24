from controller import Controller

with Controller(['python3', 'clients/python_client.py', 'vanilla'],
        client_name='vanilla_python_client',
        target_cpu_usage=.7) as controller:

    result = controller.benchmark(
        trace=True,
        with_satellites=True,
        runtime=2,
        spans_per_second=200)

    print(result)
