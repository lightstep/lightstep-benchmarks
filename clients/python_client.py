import time
import opentracing
import sys
import requests
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


CONTROLLER_PORT = 8023
SPANS_PER_LOOP = 6
SATELLITE_PORTS = [8360, 8361, 8362, 8363, 8364, 8365, 8366, 8367]

# these are much more aggressive than the defaults but are common in
# production
MAX_BUFFERED_SPANS = 10000
REPORTING_PERIOD = .2  # seconds


def do_work(units):
    i = 1.12563
    for i in range(0, units):
        i *= i


def build_tracer(command, tracer_name):
    if command['Trace'] and tracer_name == "vanilla":
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
    elif command['Trace'] and tracer_name == "cpp":
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


def generate_spans(tracer, units_work, number_spans, scope=None):
    assert number_spans >= 1

    # since python-cpp tracer doesn't allow child_of=None
    child_of_kwargs = {'child_of': scope.span} if scope else {}

    with tracer.start_active_span(
            operation_name='make_some_request',
            **child_of_kwargs) as client_scope:

        client_scope.span.set_tag('http.url', 'http://somerequesturl.com')
        client_scope.span.set_tag('http.method', "POST")
        client_scope.span.set_tag('span.kind', 'client')
        do_work(units_work)
        number_spans -= 1
        if number_spans == 0:
            return

        with tracer.start_active_span(
                operation_name='handle_some_request') as server_scope:
            server_scope.span.set_tag('http.url', 'http://somerequesturl.com')
            server_scope.span.set_tag('span.kind', 'server')
            server_scope.span.log_kv({
                'event': 'cache_miss',
                'message': 'some cache missed :('
            })

            do_work(units_work)
            number_spans -= 1
            if number_spans == 0:
                return

            with tracer.start_active_span(
                    operation_name='database_write') as db_scope:
                db_scope.span.set_tag('db.user', 'test_user')
                db_scope.span.set_tag('db.type', 'sql')
                db_scope.span.set_tag(
                    'db_statement',
                    "UPDATE ls_employees SET email = 'isaac@lightstep.com' " +
                    "WHERE employeeNumber = 27;")

                # pretend that an error happened
                db_scope.span.set_tag('error', True)
                db_scope.span.log_kv({
                    'event': 'error',
                    'stack': """File \"example.py\", line 7, in <module>
                                caller()
                                File \"example.py\", line 5, in caller
                                callee()
                                File \"example.py\", line 2, in callee
                                raise Exception(\"Yikes\")"""})

                do_work(units_work)
                number_spans -= 1
                if number_spans == 0:
                    return

            generate_spans(tracer, units_work,
                           number_spans, scope=server_scope)


def perform_work(command, tracer_name):
    logging.info("About to run this test: {}".format(command))

    tracer = build_tracer(command, tracer_name)

    sleep_debt = 0
    spans_sent = 0

    while spans_sent < command['Repeat']:
        spans_to_send = min(command['Repeat'] - spans_sent, SPANS_PER_LOOP)
        generate_spans(tracer, command['Work'], spans_to_send)
        spans_sent += spans_to_send
        sleep_debt += command['Sleep'] * spans_to_send

        if sleep_debt > command['SleepInterval']:
            sleep_debt -= command['SleepInterval']
            # 10^-9 nanoseconds / second
            time.sleep(command['SleepInterval'] * 10**-9)

    # don't include flush in time measurement
    if command['Trace'] and not command['NoFlush']:
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
        type=bool,
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
        type=bool,
        help='Whether to flush on finishing')

    args = parser.parse_args()

    while True:
        r = requests.get(f'http://localhost:{CONTROLLER_PORT}/control')
        perform_work(r.json(), args.tracer)
