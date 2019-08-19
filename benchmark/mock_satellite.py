import generated.collector_pb2 as collector
import google.protobuf
from http.server import ThreadingHTTPServer
from utils import ChunkedRequestHandler
import threading
import argparse
import sys
import random
import time
import logging

logging.basicConfig(format='%(asctime)s: %(message)s', level=logging.DEBUG, datefmt='%I:%M:%S')


# multiple threads may access spans_received so it's protected with a lock
spans_received = 0
global_lock = threading.Lock()

# fine to have this global w/o locks its not mutable
MODE = None

# all in microseconds:
ROUNDTRIP_NETWORK_LATENCY = 500 # assuming we're within the same datacenter, this is standard
TYPICAL_RESPONSE_TIME = 500
SLOW_RESPONSE_TIME = 10000
FAST_RESPONSE_TIME = 100

class SatelliteRequestHandler(ChunkedRequestHandler):
    def _send_response(self, response_code, body_string=None):
        self.send_response(response_code)

        if body_string:
            self.send_header("Content-Length", len(body_string))

        self.end_headers()

        if body_string:
            self.wfile.write((body_string).encode('utf-8'))


    # can't make a GET request to satellite server
    def GET(self):
        if self.path == "/spans_received":
            # don't need to worry about locking here since we're not going to
            # modify
            global spans_received
            logging.info(f'responded to get span request with {spans_received} spans received')
            self._send_response(200, body_string=str(spans_received))
            return
        else:
            self._send_response(400)


    def POST(self):
        if self.path == "/api/v2/reports":
            global MODE

            logging.info("starting to process report request")

            if MODE == 'typical':
                time.sleep((TYPICAL_RESPONSE_TIME + ROUNDTRIP_NETWORK_LATENCY) * 10**-6)
            if MODE == 'slow_succeed':
                time.sleep((SLOW_RESPONSE_TIME + ROUNDTRIP_NETWORK_LATENCY) * 10**-6)
            if MODE == 'slow_fail':
                time.sleep((FAST_RESPONSE_TIME + ROUNDTRIP_NETWORK_LATENCY) * 10**-6)
                self._send_response(400)
                return

            report_request = collector.ReportRequest()

            try:
                report_request.ParseFromString(self.binary_body)
            except google.protobuf.message.DecodeError as e:
                # when satellites are unable to parse the report_request, they
                # send a 500 with a brief description
                self._send_response(500, str(e))
                return

            global spans_received
            spans_in_report = len(report_request.spans)

            # aquire the global variable lock because we are using a
            # "multithreaded" server
            with global_lock:
                spans_received += spans_in_report

            logging.debug(f'read {spans_in_report} spans, total {spans_received}')

            response_string = collector.ReportResponse().SerializeToString()
            logging.info(f'response string: {response_string}')

            self._send_response(200, body_string=response_string)
        else:
            self._send_response(400)



if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Start a mock LightStep satellite.')
    parser.add_argument('port', type=int, help='port satellite will listen on')
    parser.add_argument('mode', type=str, choices=["typical", "slow_succeed", "slow_fail"], help='how the satellites will respond to requests')
    args = parser.parse_args()

    MODE = args.mode

    logging.info(f'Running satellite on port {args.port} in {args.mode} mode')

    # although this can't use "real" threading because of GIL, it can switch to
    # execute something else when we are waiting on a synchronous syscall
    httpd = ThreadingHTTPServer(('', args.port), SatelliteRequestHandler)
    httpd.serve_forever()
