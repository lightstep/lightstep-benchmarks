def pytest_addoption(parser):
    parser.addoption("--client_name", action="store")


def pytest_generate_tests(metafunc):
    # This is called for every test. Only get/set command line arguments
    # if the argument is specified in the list of test "fixturenames".
    client_name = metafunc.config.option.client_name

    if client_name:
        print(f'client_name = {client_name}')
    else:
        print("client_name argument was not passed to pytest.")

    if 'client_name' in metafunc.fixturenames and client_name is not None:
        metafunc.parametrize("client_name", [client_name])
