#!/bin/bash
#set -e

echo Python 1 CPU
./show_logs.sh python 1 | grep "CPU imp"
echo Python 2 CPUs
./show_logs.sh python 2 | grep "CPU imp"
echo Nodejs 1 CPU
./show_logs.sh nodejs 1 | grep "CPU imp"
echo Nodejs 2 CPUs
./show_logs.sh nodejs 2 | grep "CPU imp"
echo Golang 1 CPU
./show_logs.sh golang 1 | grep "CPU imp"
echo Golang 2 CPUs GOMAXPROCS=1
./show_logs.sh golang1 2 | grep "CPU imp"
