#!/bin/bash

set -e

# Name of the client, usually language or runtime (e.g., nodejs, golang).
CLIENT="$1"
CPUS="$2"
TEST_CONFIG_BASE="$3"

GCLOUD_CONFIG="devel"
CLOUD_ZONE="us-central1-a"
PROJECT_ID="lightstep-dev"
CLOUD_MACH_BASE="n1-standard-"
IMG_BASE="bench-${CLIENT}"
VM="bench-${CLIENT}-${CPUS}-${TEST_CONFIG_BASE}"

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

function dockerize()
{
    docker-machine create \
        --driver google \
        --google-project ${PROJECT_ID} \
        --google-zone ${CLOUD_ZONE} \
        --google-machine-type ${CLOUD_MACH_BASE}${CPUS} \
        ${VM} \
    || true

    eval $(docker-machine env ${VM})
    # N.B. eval ignores the return value, so check if the above worked
    if [ "$(docker-machine active)" != "${VM}" ]; then
      echo "Docker-machine failed to setup env";
      exit 1;
    fi

    local PROCS=`docker ps -q -all`
    docker kill ${PROCS} 2> /dev/null || true
    docker rm ${PROCS} 2> /dev/null || true

    docker build -t ${IMG_BASE}:latest ${DBUILD}
    docker run --name ${VM} -d ${IMG_BASE}:latest
}

trap on_exit EXIT

set_config
build
dockerize
