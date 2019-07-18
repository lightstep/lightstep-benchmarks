import subprocess
from http.server import BaseHTTPRequestHandler, HTTPServer
import threading
import json

CONTROLLER_PORT = 8023
VALID_COMMAND_KEYS = ['trace', 'repeat', 'work', 'sleep', 'no_flush', 'exit']

class CommandServer(HTTPServer):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)

        self._commands = []
        self._results = []

    """ first command in the list will be executed first """
    def add_commands(self, commands):
        # raises an exception if there is any problem with the commands
        for command in commands:
            self._validate_command(command)

        self._commands.extend(commands)

    def _validate_command(self, command):
        for key in command:
            if key not in VALID_COMMAND_KEYS:
                raise Exception(f'{key} is not a valid command field.')

        # TODO: fill this out a bit

    def next_command(self):
        if len(self._commands) == 0:
            return None

        return self._commands.pop(0)

    def get_results(self):
        return self._results.copy()

    def clear_results(self):
        self._results = []

    def add_result(self, result):
        self._results.append(result)


class RequestHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        if self.path != "/result":
            return

        # if there is a content-length header, we know how much data to read
        if "Content-Length" in self.headers:
            content_length = int(self.headers["Content-Length"])
        else:
            print("GET /result was missing a Content-Length header")
            return

        try:
            body = json.loads(self.rfile.read(content_length))
        except json.decoder.JSONDecodeError as e:
            print("Unable to parse JSON.")
            return

        self.send_response(200)
        self.end_headers()

        self.server.add_result(body)


    def do_GET(self):
        if self.path != "/control":
            return

        next_command = self.server.next_command()

        if not next_command:
            print("Client requested a command, but no more commands were available.")
            return

        self.send_response(200)
        body_string = json.dumps(next_command)
        self.send_header("Content-Length", len(body_string))
        self.end_headers()
        self.wfile.write(body_string.encode('utf-8'))


class Controller:
    def __init__(self, client_startup_args, client_name='client'):
        # start server
        self.server = CommandServer(('', CONTROLLER_PORT), RequestHandler)
        self.client_startup_args = client_startup_args
        self.client_name = client_name

    # the last one of these needs to be an exit command
    def run_tests(self, commands):
        # save commands to server, where they will be used to control stuff
        self.server.add_commands(commands)

        # startup test process
        logfile = open(f'logs/{self.client_name}.log', 'w+')
        client_handle = subprocess.Popen(self.client_startup_args, stdout=logfile, stderr=logfile)

        while len(self.server.get_results()) < len(commands):
            self.server.handle_request()

        print(self.server.get_results())
        self.server.clear_results()

        # wait for client to actually exit
        while client_handle.poll() == None:
            pass

        logfile.close()

controller = Controller(['python3', 'clients/python_template.py'], client_name='vanilla_python_client')

command = {
    'trace': False,
    'repeat': 100,
    'work': 100,
    'sleep': .001,
    'no_flush': False,
    'exit': False
}

exit_command = command.copy()
exit_command['exit'] = True

controller.run_tests([command, exit_command])
