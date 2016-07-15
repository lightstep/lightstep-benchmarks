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

DBUILD="${GOPATH}/build.$$"
SCRIPTS="${GOPATH}/../scripts"
TEST_CONFIG="${SCRIPTS}/config/${TEST_CONFIG_BASE}.json"
SCOPED="https://www.googleapis.com/auth"

GCLOUD="gcloud"
SSH="${GCLOUD} compute ssh"
DOCKER="${GCLOUD} docker --"

function usage()
{
    echo "usage: $0 title client cpus config"
    echo "  GOPATH must be set"
    echo "  Configuration in \$GOPATH/../scripts"
}

function set_config()
{
    gcloud config configurations activate ${GCLOUD_CONFIG} 2> /dev/null
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

function dockerize()
{
    if ${SSH} ${VM} true 2> /dev/null; then
	:
    else
	${GCLOUD} compute instances stop ${VM} 2> /dev/null || true
	(yes | ${GCLOUD} compute instances delete ${VM} 2> /dev/null) || true
	${GCLOUD} compute instances create ${VM} \
		  --project ${PROJECT_ID} \
		  --zone ${CLOUD_ZONE} \
		  --machine-type ${CLOUD_MACH_BASE}${CPUS} \
		  --scopes ${SCOPED}/devstorage.read_write,${SCOPED}/compute,${SCOPED}/cloud-platform \
		  --image-family client-benchmarks \
		  --boot-disk-auto-delete
    fi

    local PROCS=`${DOCKER} ps -q -all`
    ${DOCKER} kill ${PROCS} 2> /dev/null || true
    ${DOCKER} rm ${PROCS} 2> /dev/null || true

    ${DOCKER} build -t ${IMG_BASE}:latest ${DBUILD}
    ${DOCKER} run \
	   -d \
	   -e BENCHMARK_CONFIG=${TEST_CONFIG_BASE} \
	   -e BENCHMARK_TITLE=${TITLE} \
	   -e BENCHMARK_BUCKET=${BUCKET} \
	   -e BENCHMARK_ZONE=${CLOUD_ZONE} \
	   -e BENCHMARK_PROJECT=${PROJECT_ID} \
	   -e BENCHMARK_INSTANCE=${VM} \
	   --name ${VM} \
	   ${IMG_BASE}:latest

    # Note: the controller deletes its own VM
}

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

if [ ! -d "${SCRIPTS}" ]; then
    echo "Scripts directory not found (${SCRIPTS})"
    exit 1
fi

trap on_exit EXIT

set_config

# TODO Refactor this code to build once, then run one VM per client test
build
dockerize
