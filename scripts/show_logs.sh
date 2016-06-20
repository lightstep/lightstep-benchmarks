#!/bin/bash

set -e

CLIENT=${1}
CPUS=${2}

eval $(docker-machine env ls-bench-${CLIENT}-${CPUS})

docker logs bench-${CLIENT}-${CPUS} 2>&1
