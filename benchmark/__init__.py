from .utils import setup_file_logger
import logging

benchmark_logger = logging.getLogger(__name__)
setup_file_logger(benchmark_logger, 'benchmark.log')
