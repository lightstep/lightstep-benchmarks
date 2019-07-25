from controller import Controller

try:
    with Controller(['python3', 'clients/python_client.py', 'cpp'],
            client_name='cpp_python_client',
            target_cpu_usage=.7,
            num_satellites=8) as controller:

        result = controller.benchmark(
            trace=True,
            with_satellites=True,
            runtime=10,
            spans_per_second=200)

        print(result)

except Exception as e:
    with open('logs/cpp_python_client.log') as file:
        print(file.read())

    print("exception::")
    print(e)
