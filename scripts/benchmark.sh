#!/bin/bash

set -e

# Name of the client, usually language or runtime (e.g., nodejs, golang).
CLIENT="$1"

CLOUD_ZONE="us-central1-a"
PROJECT_ID="helpful-cat-109717"
CLOUD_MACH_BASE="n1-standard-"
VM_BASE="ls-bench-${CLIENT}-"
IMG_BASE="bench-${CLIENT}"

if [ "${CLIENT}" = "" ]; then
    echo "usage: $0 client_name"
    exit 1
fi

if [ ! -d "${GOPATH}/src" ]; then
    echo "GOPATH must be set"
    exit 1
fi

DBUILD="${GOPATH}/build.$$"
SCRIPTS="${GOPATH}/../scripts"

if [ ! -d "${SCRIPTS}" ]; then
    echo "Scripts directory not found (${SCRIPTS})"
    exit 1
fi

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
    rm -rf "$DBUILD"
}

function dockerize()
{
    local CONC=1
    local CPUS=$[$CONC+1]
    local VM="${VM_BASE}${CPUS}"

    docker-machine create \
        --driver google \
        --google-project ${PROJECT_ID} \
        --google-zone ${CLOUD_ZONE} \
        --google-machine-type ${CLOUD_MACH_BASE}${CPUS} \
        ${VM}

    eval $(docker-machine env ${VM})

    docker build -t ${IMG_BASE}:latest ${DBUILD}
    docker run -d ${IMG_BASE}:latest
}

trap on_exit EXIT

build
dockerize

