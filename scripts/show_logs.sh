#!/bin/bash

set -e

if [ -z "${PROJECT_ID}" ]; then
    echo Please set PROJECT_ID
    exit 1
fi

GCLOUD=gcloud
GCLOUD_CONFIG=devel

function set_config()
{
    local config=`${GCLOUD} config configurations list | grep True | awk '{print $1}'`
    if [ ${config} != ${GCLOUD_CONFIG} ]; then
	# TODO make the gcloud project/zone parameters instead
	echo "Please run:"
	echo "  ${GCLOUD} config configurations activate ${GCLOUD_CONFIG}"
	exit 1
    fi
}

TITLE=${1}
CLIENT=${2}
CPUS=${3}
TEST_CONFIG_BASE=${4}
CLOUD_ZONE="us-central1-a"
SSH="${GCLOUD} compute ssh --project ${PROJECT_ID} --zone ${CLOUD_ZONE}"
MACHINE=bench-${TITLE}-${CLIENT}-${CPUS}-${TEST_CONFIG_BASE}

set_config

${SSH} ${MACHINE} -- sudo docker logs ${MACHINE} 2>&1
