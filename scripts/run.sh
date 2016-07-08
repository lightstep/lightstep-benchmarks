#!/bin/bash
set -e

# Concurrency=1
# ./benchmark.sh python 1 single-thread-zero-logs
# ./benchmark.sh python 2 single-thread-zero-logs
# ./benchmark.sh nodejs 1 single-thread-zero-logs
# ./benchmark.sh nodejs 2 single-thread-zero-logs
# ./benchmark.sh golang 1 single-thread-zero-logs
# ./benchmark.sh golang1 2 single-thread-zero-logs

# Concurrency>1
./benchmark.sh golang 2 two-threads-zero-logs
./benchmark.sh golang 4 four-threads-zero-logs
./benchmark.sh golang 16 sixteen-threads-zero-logs

./benchmark.sh python 2 two-threads-zero-logs
./benchmark.sh python 4 four-threads-zero-logs

# Concurrent w/ logs
./benchmark.sh golang 2 two-threads-1kb-logs
./benchmark.sh golang 4 four-threads-1kb-logs
./benchmark.sh golang 16 sixteen-threads-1kb-logs

./benchmark.sh python 2 two-threads-1kb-logs
./benchmark.sh python 4 four-threads-1kb-logs

# Logs
./benchmark.sh golang 1 single-thread-1kb-logs
./benchmark.sh golang 1 single-thread-5kb-logs
./benchmark.sh golang 1 single-thread-10kb-logs
