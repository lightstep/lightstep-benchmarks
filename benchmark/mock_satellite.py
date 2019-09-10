import generated.collector_pb2 as collector
import google.protobuf
from http.server import ThreadingHTTPServer
from utils import ChunkedRequestHandler
import threading
import argparse
import time
import logging
import sys

# log everything with no format, because these messages will be formatted
# and printed by the controller
# log to stdout so that the controller can differentiate between errors
# (written to stderr) and logs (written to stdout)
logging.basicConfig(
    format='%(message)s',
    level=logging.DEBUG,
    handlers=[logging.StreamHandler(sys.stdout)])

# multiple threads may access spans_received so it's protected with a lock
spans_received = 0
global_lock = threading.Lock()

# fine to have this global w/o locks its not mutable
MODE = None

# all in microseconds:

SPAN_NORMALIZER = 10000.0
TYPICAL_RESPONSE_TIME = 500 / SPAN_NORMALIZER
SLOW_RESPONSE_TIME = 10000 / SPAN_NORMALIZER
FAST_RESPONSE_TIME = 100 / SPAN_NORMALIZER


class SatelliteRequestHandler(ChunkedRequestHandler):
    def _send_response(self, response_code, body_string=None):
        self.send_response(response_code)

        if body_string:
            self.send_header("Content-Length", len(body_string))

        self.end_headers()

        if body_string:
            self.wfile.write((body_string).encode('utf-8'))

    def log_message(self, message, *args):
        logging.info(message % tuple(args))

    def GET(self):
        if self.path == "/spans_received":
            # don't need to worry about locking here since we're not going to
            # modify
            global spans_received
            logging.info(("Responded that {} spans have been received. " +
                          "This number does NOT count resets.").format(
                          spans_received))

            self._send_response(200, body_string=str(spans_received))
            return
        else:
            self._send_response(400)

    def POST(self):
        if self.path == "/api/v2/reports":
            global MODE

            logging.info("Processing report request in {} mode.".format(MODE))

            report_request = collector.ReportRequest()

            try:
                report_request.ParseFromString(self.binary_body)
                spans_in_report = len(report_request.spans)
                if MODE == 'typical':
                    time.sleep(
                        (TYPICAL_RESPONSE_TIME*spans_in_report) * 10**-6)
                if MODE == 'slow_succeed':
                    time.sleep((SLOW_RESPONSE_TIME*spans_in_report) * 10**-6)
                if MODE == 'slow_fail':
                    time.sleep((FAST_RESPONSE_TIME*spans_in_report) * 10**-6)
                    self._send_response(400)
                    return
            except google.protobuf.message.DecodeError as e:
                # when satellites are unable to parse the report_request, they
                # send a 500 with a brief description
                self._send_response(500, str(e))
                return

            global spans_received

            # aquire the global variable lock because we are using a
            # "multithreaded" server
            with global_lock:
                spans_received += spans_in_report

            logging.debug('Report Request contained {} spans.'.format(
                spans_in_report, spans_received))

            # Right now `response_string` is actually just b'', but we can
            # improve this is need be to make a more sophisticated response
            response_string = collector.ReportResponse().SerializeToString()

            self._send_response(200, body_string=response_string)
        else:
            self._send_response(400)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(
        description='Start a mock LightStep satellite.')
    parser.add_argument('port',
                        type=int,
                        help='port satellite will listen on')
    parser.add_argument('mode',
                        type=str,
                        choices=["typical", "slow_succeed", "slow_fail"],
                        help='how the satellites will respond to requests')
    args = parser.parse_args()

    MODE = args.mode

    logging.info(f'Running satellite on port {args.port} in {args.mode} mode')

    # although this can't use "real" threading because of GIL, it can switch to
    # execute something else when we are waiting on a synchronous syscall
    httpd = ThreadingHTTPServer(('', args.port), SatelliteRequestHandler)
    httpd.serve_forever()
