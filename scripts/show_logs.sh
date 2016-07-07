#!/bin/bash

set -e

CLIENT=${1}
CPUS=${2}
TEST_CONFIG_BASE=${3}
MACHINE=bench-${CLIENT}-${CPUS}-${TEST_CONFIG_BASE}

eval $(docker-machine env ${MACHINE})

docker logs ${MACHINE} 2>&1
