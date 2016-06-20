#!/bin/bash

set -e

# Name of the client, usually language or runtime (e.g., nodejs, golang).
CLIENT="$1"
CPUS="$2"

CLOUD_ZONE="us-central1-a"
PROJECT_ID="helpful-cat-109717"
CLOUD_MACH_BASE="n1-standard-"
VM_BASE="ls-bench-${CLIENT}-"
IMG_BASE="bench-${CLIENT}"
CONTAINER="${IMG_BASE}-${CPUS}"

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

if [ ! -d "${SCRIPTS}" ]; then
    echo "Scripts directory not found (${SCRIPTS})"
    exit 1
fi

function usage()
{
    echo "usage: $0 client_name cpus"
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
}

function on_exit()
{
    rm -rf "${DBUILD}"
}

function dockerize()
{
    local VM="${VM_BASE}${CPUS}"

    docker-machine create \
        --driver google \
        --google-project ${PROJECT_ID} \
        --google-zone ${CLOUD_ZONE} \
        --google-machine-type ${CLOUD_MACH_BASE}${CPUS} \
        ${VM} \
    || true

    eval $(docker-machine env ${VM})

    PROCS=`docker ps -q`
    docker kill ${PROCS} 2> /dev/null || true
    docker rm ${PROCS} 2> /dev/null || true

    docker build -t ${IMG_BASE}:latest ${DBUILD}
    docker run --name ${CONTAINER} -d ${IMG_BASE}:latest
}

trap on_exit EXIT

build
dockerize

