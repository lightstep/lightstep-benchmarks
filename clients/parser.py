import argparse

def get_args():
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

    return parser.parse_args()
