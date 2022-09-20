import time
import logging

from opentelemetry import context, trace
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import (
    BatchSpanProcessor,
    ConsoleSpanExporter,
)
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from parser import get_args

SPANS_PER_LOOP = 6
SATELLITE_PORTS = [8360, 8361, 8362, 8363, 8364, 8365, 8366, 8367]

# these are much more aggressive than the defaults but are common in
# production
MAX_BUFFERED_SPANS = 10000
REPORTING_PERIOD = .2  # seconds

args = None

tags = None
logs = None


def setup_annotations():
    global tags
    global logs
    tags = {}
    for i in range(args.num_tags):
        tags['tag.key%d' % i] = 'tag.value%d' % i
    logs = []
    for i in range(args.num_logs):
        logs.append(('log.key%d' % i, 'log.value%d' % i))

def do_work(units):
    i = 1.12563
    for i in range(0, units):
        i *= i

def build_tracer():
    provider = TracerProvider()
    processor = BatchSpanProcessor(OTLPSpanExporter(endpoint="localhost:8360", headers=(("lightstep-access-token", "developer"),), insecure=True))
    provider.add_span_processor(processor)

    # Sets the global default tracer provider
    trace.set_tracer_provider(provider)
    return trace.get_tracer(__name__)

def generate_spans(tracer, units_work, number_spans, parent_ctx=None):
    assert number_spans >= 1
    with tracer.start_as_current_span("span-name", attributes=tags, context=parent_ctx):
        do_work(units_work)
        number_spans -= 1
        if number_spans == 0:
            return
        with tracer.start_as_current_span("span-name"):
            do_work(units_work)
            number_spans -= 1
            if number_spans == 0:
                return
            with tracer.start_as_current_span("span-name"):
                do_work(units_work)
                number_spans -= 1
                if number_spans == 0:
                    return
            generate_spans(tracer, units_work,
                           number_spans, parent_ctx=context.get_current())


def perform_work():
    logging.info("About to run this test: {}".format(args))

    tracer = build_tracer()

    sleep_debt = 0
    spans_sent = 0

    while spans_sent < args.repeat:
        spans_to_send = min(args.repeat - spans_sent, SPANS_PER_LOOP)
        generate_spans(tracer, args.work, spans_to_send)
        spans_sent += spans_to_send
        sleep_debt += args.sleep * spans_to_send

        if sleep_debt > args.sleep_interval:
            sleep_debt -= args.sleep_interval
            # 10^-9 nanoseconds / second
            time.sleep(args.sleep_interval * 10**-9)

    # don't include flush in time measurement
    if args.trace and not args.no_flush:
        logging.info("Flushing spans.")
        # tracer.flush()
    logging.info("Exiting.")
    exit()


if __name__ == '__main__':
    
    args = get_args()

    setup_annotations()

    while True:
        perform_work()
