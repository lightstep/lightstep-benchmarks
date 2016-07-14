#!/bin/bash

set -e

TITLE=${1}
CLIENT=${2}
CPUS=${3}
TEST_CONFIG_BASE=${4}
MACHINE=bench-${TITLE}-${CLIENT}-${CPUS}-${TEST_CONFIG_BASE}

eval $(docker-machine env ${MACHINE})

docker logs ${MACHINE} 2>&1
