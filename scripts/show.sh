#!/bin/bash
#set -e
# TODO this is a throw-away script. nothing to see here.

# echo Python 1x1
# ./show_logs.sh python 1 single-thread-zero-logs | grep "CPU imp"
# echo Python 1x2
# ./show_logs.sh python 2 single-thread-zero-logs | grep "CPU imp"
# echo Nodejs 1x1
# ./show_logs.sh nodejs 1 single-thread-zero-logs | grep "CPU imp"
# echo Nodejs 1x2
# ./show_logs.sh nodejs 2 single-thread-zero-logs | grep "CPU imp"
# echo Golang 1x1
# ./show_logs.sh golang 1 single-thread-zero-logs | grep "CPU imp"
# echo Golang 1x2 GOMAXPROCS=1
# ./show_logs.sh golang1 2 single-thread-zero-logs | grep "CPU imp"

echo Concurrency>1

echo Golang 2
./show_logs.sh golang 2 two-threads-zero-logs | grep "CPU imp"

echo Golang 4
./show_logs.sh golang 4 four-threads-zero-logs | grep "CPU imp"

echo Golang 16
./show_logs.sh golang 16 sixteen-threads-zero-logs | grep "CPU imp"

echo Python 2
./show_logs.sh python 2 two-threads-zero-logs | grep "CPU imp"
