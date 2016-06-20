#!/bin/bash

set -e

CLIENT=${1}
CPUS=${2}

eval $(docker-machine env ls-bench-${CLIENT}-${CPUS})

#docker ps --all
docker logs bench-${CLIENT}-${CPUS}
