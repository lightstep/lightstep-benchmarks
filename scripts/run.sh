#!/bin/bash
set -e

./benchmark.sh python 1
./benchmark.sh python 2
./benchmark.sh nodejs 1
./benchmark.sh nodejs 2
./benchmark.sh golang 1
./benchmark.sh golang1 2

