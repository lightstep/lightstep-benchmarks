import psutil
import time
import opentracing
import sys
import requests
import argparse
import logging

# log everything with no format, because these messages will be formatted
# and printed by the controller
logging.basicConfig(format='%(message)s', level=logging.DEBUG)


MEMORY_PERIOD = 1  # report memory use every 5 seconds
CONTROLLER_PORT = 8023
NUM_SATELLITES = 8
MAX_BUFFERED_SPANS = 10000
REPORTING_PERIOD = 200  # ms
SPANS_PER_LOOP = 6


def do_work(units):
    i = 1.12563
    for i in range(0, units):
        i *= i


def send_result(result):
    requests.get(f'http://localhost:{CONTROLLER_PORT}/result', params=result)


class Monitor:
    """ Special timer to measure process time and time spent as a result of
    this process' system calls.

    Records 2 * 10^-5 seconds when we immediately run start() then stop(), so
    tests should be at ms scale to dwarf this contribution.
    """
    def __init__(self):
        self.process = psutil.Process()

    def get_memory(self):
        """ Returns the size of process virtual memory """
        return self.process.memory_info()[0]

    def start(self):
        user, system, _, _ = self.process.cpu_times()
        self.start_cpu_time = user + system
        self.start_clock_time = time.time()
        self.get_cpu()

    def get_cpu(self):
        """ Gets CPU %, calculated since last call to split. """
        return self.process.cpu_percent(interval=None)

    def stop(self):
        user, system, _, _ = self.process.cpu_times()
        return (user + system - self.start_cpu_time,
                time.time() - self.start_clock_time)


def build_tracer(command, tracer_name, port):
    if command['Trace'] and tracer_name == "vanilla":
        logging.info("We're using the python tracer.")
        import lightstep
        return lightstep.Tracer(
            component_name='isaac_service',
            collector_port=port,
            collector_host='localhost',
            collector_encryption='none',
            use_http=True,
            access_token='developer',
            # these are much more aggressive than the defaults
            # but are common in production
            periodic_flush_seconds=REPORTING_PERIOD / 1000,
            max_span_records=MAX_BUFFERED_SPANS,
        )
    elif command['Trace'] and tracer_name == "cpp":
        logging.info("We're using the python-cpp tracer.")
        import lightstep_native
        return lightstep_native.Tracer(
            component_name='isaac_service',
            access_token='developer',
            use_stream_recorder=True,
            collector_plaintext=True,
            satellite_endpoints=[{'host': 'localhost', 'port': p}
                                 for p in range(port, port + NUM_SATELLITES)],
            max_buffered_spans=MAX_BUFFERED_SPANS,
            reporting_period=REPORTING_PERIOD,
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


def perform_work(command, tracer_name, port):
    logging.info("About to run this test: {}".format(command))
    logging.info("Connecting to satellite on port {}".format(port))

    # if exit is set to true, end the program
    if command['Exit']:
        send_result({})
        logging.info("sent exit response, now exiting...")
        sys.exit()

    tracer = build_tracer(command, tracer_name, port)

    sleep_debt = 0
    spans_sent = 0

    last_memory_save = time.time()
    memory_list = []
    cpu_list = []

    monitor = Monitor()
    monitor.start()

    while spans_sent < command['Repeat']:
        spans_to_send = min(command['Repeat'] - spans_sent, SPANS_PER_LOOP)
        generate_spans(tracer, command['Work'], spans_to_send)
        spans_sent += spans_to_send
        sleep_debt += command['Sleep'] * spans_to_send

        if sleep_debt > command['SleepInterval']:
            sleep_debt -= command['SleepInterval']
            # 10^-9 nanoseconds / second
            time.sleep(command['SleepInterval'] * 10**-9)

        if time.time() > last_memory_save + MEMORY_PERIOD:
            memory_list.append(monitor.get_memory())
            # saves CPU percentage as fraction since last call
            cpu_list.append(monitor.get_cpu() / 100)
            last_memory_save = time.time()

    memory_list.append(monitor.get_memory())

    # don't include flush in time measurement
    if command['Trace'] and not command['NoFlush']:
        logging.info("Flushing spans.")
        tracer.flush()

    cpu_time, clock_time = monitor.stop()

    result = {
        'ProgramTime': cpu_time,
        'ClockTime': clock_time,
        'SpansSent': spans_sent,
        'MemoryList': memory_list,
        'CPUList': cpu_list,
    }

    logging.info("Sending result to controller:")
    logging.info(result)
    send_result(result)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(
        description='Start a client to test a LightStep tracer.')

    parser.add_argument(
        'port',
        type=int,
        help='Which port to connect to the satellite on.')

    parser.add_argument(
        'tracer',
        type=str,
        choices=["vanilla", "cpp"],
        help='Which LightStep tracer to use.')

    args = parser.parse_args()

    while True:
        r = requests.get(f'http://localhost:{CONTROLLER_PORT}/control')
        perform_work(r.json(), args.tracer, args.port)
