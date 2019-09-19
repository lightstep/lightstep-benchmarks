import time
import opentracing
import sys
import argparse
import logging

# log everything with no format, because these messages will be formatted
# and printed by the controller
# log to stdout so that the controller can differentiate between errors
# (written to stderr) and logs (written to stdout)
logging.basicConfig(
    format='%(message)s',
    level=logging.DEBUG,
    handlers=[logging.StreamHandler(sys.stdout)])


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
    tags = []
    for i in range(args.num_tags):
        tags.append(('tag.key%d' % i, 'tag.value%d' % i))
    logs = []
    for i in range(args.num_logs):
        logs.append(('log.key%d' % i, 'log.value%d' % i))


def do_work(units):
    i = 1.12563
    for i in range(0, units):
        i *= i


def build_tracer():
    tracer_name = args.tracer
    if args.trace and tracer_name == "vanilla":
        logging.info("We're using the python tracer.")
        import lightstep
        return lightstep.Tracer(
            component_name='isaac_service',
            collector_port=SATELLITE_PORTS[0],
            collector_host='localhost',
            collector_encryption='none',
            use_http=True,
            access_token='developer',
            periodic_flush_seconds=REPORTING_PERIOD,
            max_span_records=MAX_BUFFERED_SPANS,
        )
    elif args.trace and tracer_name == "cpp":
        logging.info("We're using the python-cpp tracer.")
        import lightstep_streaming
        return lightstep_streaming.Tracer(
            component_name='isaac_service',
            access_token='developer',
            use_stream_recorder=True,
            collector_plaintext=True,
            satellite_endpoints=[{'host': 'localhost', 'port': p}
                                 for p in SATELLITE_PORTS],
            max_buffered_spans=MAX_BUFFERED_SPANS,
            reporting_period=REPORTING_PERIOD * 10**6,  # s --> us
        )

    logging.info("We're using a NoOp tracer.")
    return opentracing.Tracer()


def make_scope(tracer, parent=None):
    if parent:
        scope = tracer.start_active_span('isaac_service', child_of=parent)
    else:
        scope = tracer.start_active_span('isaac_service')
    for key, val in tags:
        scope.span.set_tag(key, val)
    for key, val in logs:
        scope.span.log_kv({key: val})
    return scope


def generate_spans(tracer, units_work, number_spans, parent=None):
    assert number_spans >= 1

    with make_scope(tracer, parent):
        do_work(units_work)
        number_spans -= 1
        if number_spans == 0:
            return
        with make_scope(tracer) as server_scope:
            do_work(units_work)
            number_spans -= 1
            if number_spans == 0:
                return
            with make_scope(tracer):
                do_work(units_work)
                number_spans -= 1
                if number_spans == 0:
                    return
            generate_spans(tracer, units_work,
                           number_spans, parent=server_scope.span)


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
        tracer.flush()

    logging.info("Exiting.")
    exit()


if __name__ == '__main__':
    parser = argparse.ArgumentParser(
        description='Start a client to test a LightStep tracer.')

    parser.add_argument(
        'tracer',
        type=str,
        choices=["vanilla", "cpp"],
        help='Which LightStep tracer to use.')
    parser.add_argument(
        '--trace',
        type=int,
        help='Whether to trace')
    parser.add_argument(
        '--sleep',
        type=float,
        help='The amount of time to sleep for each span')
    parser.add_argument(
        '--sleep_interval',
        type=float,
        help='The duration for each sleep')
    parser.add_argument(
        '--work',
        type=int,
        help='The quantity of work to perform between spans')
    parser.add_argument(
        '--repeat',
        type=int,
        help='The number of span generation repetitions to perform')
    parser.add_argument(
        '--no_flush',
        type=int,
        help='Whether to flush on finishing')
    parser.add_argument(
        '--num_tags',
        type=int,
        help='The number of tags to annotate spans with')
    parser.add_argument(
        '--num_logs',
        type=int,
        help='The number of logs to annotate spans with')

    args = parser.parse_args()

    setup_annotations()

    while True:
        perform_work()
