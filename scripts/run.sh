#!/bin/bash
set -e

## No logging benchmarks

# Concurrency=1
./benchmark.sh python 1 single-thread-zero-logs
./benchmark.sh python 2 single-thread-zero-logs
./benchmark.sh nodejs 1 single-thread-zero-logs
./benchmark.sh nodejs 2 single-thread-zero-logs
./benchmark.sh golang 1 single-thread-zero-logs
./benchmark.sh golang1 2 single-thread-zero-logs

# Concurrency=2
./benchmark.sh golang 2 two-threads-zero-logs
./benchmark.sh golang 4 two-threads-zero-logs
