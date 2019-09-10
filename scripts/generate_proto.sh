#!/bin/bash

set -e

SRC_FOLDER="proto"
DST_FOLDER="generated"

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
BENCHMARK_DIR="$SCRIPT_DIR/../benchmark"

# now compile proto files --> python files
mkdir -p ${BENCHMARK_DIR}/generated
protoc -I=${BENCHMARK_DIR}/${SRC_FOLDER} --python_out=${BENCHMARK_DIR}/${DST_FOLDER} ${BENCHMARK_DIR}/${SRC_FOLDER}/collector.proto
