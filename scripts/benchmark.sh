#!/bin/bash

set -e

TITLE=${1}
CLIENT=${2}
CPUS=${3}
TEST_CONFIG_BASE=${4}

BUCKET="lightstep-client-benchmarks"
GCLOUD_CONFIG="devel"
CLOUD_ZONE="us-central1-a"
PROJECT_ID="lightstep-dev"
CLOUD_MACH_BASE="n1-standard-"
IMG_BASE="bench-${CLIENT}"
VM="bench-${TITLE}-${CLIENT}-${CPUS}-${TEST_CONFIG_BASE}"

STANDING_GCLOUD_CONFIG=`gcloud config configurations list | grep True | awk '{print $1}'`

if [ "${CLIENT}" = "" ]; then
    usage
    exit 1
fi
if [ "${CPUS}" = "" ]; then
    usage
    exit 1
fi

if [ ! -d "${GOPATH}/src" ]; then
    usage
    exit 1
fi

DBUILD="${GOPATH}/build.$$"
SCRIPTS="${GOPATH}/../scripts"
TEST_CONFIG="${SCRIPTS}/config/${TEST_CONFIG_BASE}.json"

if [ ! -d "${SCRIPTS}" ]; then
    echo "Scripts directory not found (${SCRIPTS})"
    exit 1
fi

function set_config()
{
    gcloud config configurations activate ${GCLOUD_CONFIG} 2> /dev/null
}

function usage()
{
    echo "usage: $0 client_name cpus config"
    echo "  GOPATH must be set"
    echo "  Configuration in \$GOPATH/../scripts"
}

function build()
{
    rm -rf ${DBUILD}
    mkdir ${DBUILD}

    (GOOS=linux GOARCH=amd64 go build -o ${DBUILD}/controller $GOPATH/src/controller.go)

    . ${SCRIPTS}/docker/${CLIENT}.sh

    ln ${SCRIPTS}/docker/Dockerfile.${CLIENT} ${DBUILD}/Dockerfile
    ln ${TEST_CONFIG} ${DBUILD}/config.json
}

function on_exit()
{
    rm -rf "${DBUILD}"
    gcloud config configurations activate ${STANDING_GCLOUD_CONFIG} 2> /dev/null
}

function create_machine()
{
    docker-machine create \
		   --driver google \
		   --google-project ${PROJECT_ID} \
		   --google-zone ${CLOUD_ZONE} \
		   --google-machine-type ${CLOUD_MACH_BASE}${CPUS} \
		   --google-scopes https://www.googleapis.com/auth/devstorage.read_write \
		   ${VM}
}

function dockerize()
{
    if docker-machine ssh ${VM} true; then
	:
    else
	docker-machine stop ${VM} || true
	docker-machine rm -f ${VM} || true
	create_machine
    fi

    eval $(docker-machine env ${VM})	
    if [ "$(docker-machine active)" != "${VM}" ]; then
	echo "Docker-machine failed to setup env"
	exit 1
    fi

    local PROCS=`docker ps -q -all`
    docker kill ${PROCS} 2> /dev/null || true
    docker rm ${PROCS} 2> /dev/null || true

    docker build -t ${IMG_BASE}:latest ${DBUILD}
    docker run \
	   -d \
	   -e BENCHMARK_CONFIG=${TEST_CONFIG_BASE} \
	   -e BENCHMARK_TITLE=${TEST_TITLE} \
	   -e BENCHMARK_BUCKET=${BUCKET} \
	   --name ${VM} \
	   ${IMG_BASE}:latest

    # TODO remove -d above
    # docker-machine rm ${VM}
}

trap on_exit EXIT

set_config

# TODO Refactor this code to build once, then run one VM per client test
build
dockerize
