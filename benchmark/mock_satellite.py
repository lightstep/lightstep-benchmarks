import generated.collector_pb2 as collector
import google.protobuf
from http.server import ThreadingHTTPServer
from utils import ChunkedRequestHandler
import threading
import argparse
import time
import logging
import os
import sys

from opentelemetry import metrics
from opentelemetry.exporter.otlp.proto.grpc.metric_exporter import OTLPMetricExporter
from opentelemetry.sdk.metrics import MeterProvider
from opentelemetry.sdk.metrics.export import (
    ConsoleMetricExporter,
    PeriodicExportingMetricReader,
)
from opentelemetry.sdk.resources import SERVICE_NAME, Resource

from opentelemetry.proto.collector.trace.v1.trace_service_pb2 import ExportTraceServiceResponse, ExportTraceServiceRequest

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
ATTRS = {}

# all in microseconds:

SPAN_NORMALIZER = 10000.0
TYPICAL_RESPONSE_TIME = 500 / SPAN_NORMALIZER
SLOW_RESPONSE_TIME = 10000 / SPAN_NORMALIZER
FAST_RESPONSE_TIME = 100 / SPAN_NORMALIZER

def get_meter():
  token = os.environ.get("LS_ACCESS_TOKEN")
  if token is None:
    print("LS_ACCESS_TOKEN must be set")
    sys.exit(1)
  metric_reader = PeriodicExportingMetricReader(OTLPMetricExporter(endpoint="ingest.lightstep.com:443", headers=(("lightstep-access-token", token),)))
  resource = Resource(attributes={
      SERVICE_NAME: "mock-satellite"
  })

  provider = MeterProvider(metric_readers=[metric_reader], resource=resource)
  metrics.set_meter_provider(provider)
  return metrics.get_meter(__name__)

ATTRS
spans_received_counter = get_meter().create_counter(
    "benchmark.spans_received", unit="1", description="Counts the number of received spans"
)

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
        print(self.path)
        if self.path == "/api/v2/reports":
            global MODE
            global ATTRS

            logging.info("Processing report request in {} mode.".format(MODE))

            report_request = collector.ReportRequest()

            try:
                report_request.ParseFromString(self.binary_body)
                spans_in_report = len(report_request.spans)
                spans_received_counter.add(spans_in_report, attributes=ATTRS)
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
        elif self.path == "/v1/traces":
            report_request = ExportTraceServiceRequest()

            try:
                report_request.ParseFromString(self.binary_body)
                # from ipdb import set_trace
                # set_trace()
                spans_in_report = 0
                for rspan in report_request.resource_spans:
                    for scope in rspan.scope_spans:
                        spans_in_report += len(scope.spans)
                
                # print(report_request.ScopeSpans().Spans())
                # spans_in_report = len(report_request.spans)
                spans_received_counter.add(spans_in_report, attributes=ATTRS)
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
            response_string = ExportTraceServiceResponse().SerializeToString()
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
    parser.add_argument('tracer',
                        type=str,
                        choices=["lightstep-tracer-go", "lightstep-tracer-python", "otel-python", "otel-go"],
                        help='what tracer is sending spans')
    args = parser.parse_args()

    MODE = args.mode
    ATTRS = {"tracer": args.tracer}

    logging.info(f'Running satellite on port {args.port} in {args.mode} mode')

    httpd = ThreadingHTTPServer(
        ('0.0.0.0', args.port),
        SatelliteRequestHandler
    )
    httpd.serve_forever()
