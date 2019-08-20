from http.server import BaseHTTPRequestHandler
from os import path, makedirs
import subprocess
from threading import Thread
import logging
import sys

BENCHMARK_DIR = path.dirname(path.realpath(__file__))
PROJECT_DIR = path.join(BENCHMARK_DIR, "..")


def start_logging_subprocess(cli_args, logger):
    # send stdout and stderr to different pipes
    # TODO: print stdout with logging.info and stderr with logging.error
    handler = subprocess.Popen(
        cli_args,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT)  # send output to stdout / stderr pipes

    stdout_logging_thread = Thread(
        target=_log_output,
        args=(handler, logger))
    stdout_logging_thread.daemon = True
    stdout_logging_thread.start()

    return handler


def _log_output(process_handler, logger):
    # read until we reach ''
    for line in iter(process_handler.stdout.readline, b''):
        logger.info(line.decode('ascii')[:-1])  # last char is \n, ignore this


def setup_logger(logger, filename):
    basic_formatter = logging.Formatter(
        fmt="%(levelname)s %(name)s %(asctime)s: %(message)s",
        datefmt='%H:%M:%S',
        style='%')

    # create the logs folder
    logs_dir = path.join(PROJECT_DIR, 'logs')
    logs_file = path.join(logs_dir, filename)
    makedirs(logs_dir, exist_ok=True)

    # output ALL logs to a file
    file_handler = logging.FileHandler(logs_file, mode='a')
    file_handler.setFormatter(basic_formatter)
    file_handler.setLevel(logging.DEBUG)

    print_handler = logging.StreamHandler(stream=sys.stdout)
    print_handler.setFormatter(basic_formatter)
    print_handler.setLevel(logging.WARNING)

    logger.setLevel(logging.DEBUG)
    logger.addHandler(file_handler)
    logger.addHandler(print_handler)


class ChunkedRequestHandler(BaseHTTPRequestHandler):
    # A class that extends BaseHTTPRequestHandler to support chunked encoding.
    # The class will read POST request headers and determine if the request is
    # in fixed-length or chunked format. The request body will be parsed and
    # saved in `binary_body` bytearray.
    # Derrived classes should use POST and GET instead of do_POST and do_GET.

    def do_POST(self):
        # if there is a content-length header, we know how much data to read
        if "Content-Length" in self.headers:
            content_length = int(self.headers["Content-Length"])
            self.binary_body = self.rfile.read(content_length)

        # see http://en.wikipedia.org/wiki/Chunked_transfer_encoding
        elif 'Transfer-Encoding' in self.headers and \
                self.headers['Transfer-Encoding'] == 'chunked':

            self.binary_body = bytearray()

            while True:
                # chunk begind with [length hex]\r\n
                read_len = self._read_chunk_length()

                # when there is a 0-length chunk we are done
                if read_len <= 0:
                    break

                # read appropriate number of bytes
                binary_chunk = self.rfile.read(read_len)
                self.binary_body += binary_chunk

                # chunk ends with /r/n
                self._read_delimiter()

        else:
            raise Exception(
                "Missing Content-Length or Transfer-Encoding headers")

        self.POST()

    def do_GET(self):
        self.GET()

    def POST(self):
        pass

    def GET(self):
        pass

    def _read_delimiter(self, delimiter=b'\r\n'):
        bytes_read = self.rfile.read(len(delimiter))

        if bytes_read != delimiter:
            raise Exception("Unable to read delimiter.")

    def _read_chunk_length(self, delimiter=b'\r\n', max_bytes=16):
        buf = bytearray()
        delim_len = len(delimiter)

        while len(buf) < max_bytes:
            c = self.rfile.read(1)

            buf += c

            if buf[-delim_len:] == delimiter:

                try:
                    return int(bytes(buf[:-delim_len]), 16)
                except ValueError:
                    return -1

        return -1
