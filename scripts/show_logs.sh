#!/bin/bash

set -e

TITLE=${1}
CLIENT=${2}
CPUS=${3}
TEST_CONFIG_BASE=${4}
CLOUD_ZONE="us-central1-a"
PROJECT_ID="lightstep-dev"
GCLOUD=gcloud
SSH="${GCLOUD} compute ssh --project ${PROJECT_ID} --zone ${CLOUD_ZONE}"
MACHINE=bench-${TITLE}-${CLIENT}-${CPUS}-${TEST_CONFIG_BASE}

${SSH} ${MACHINE} -- sudo docker logs ${MACHINE} 2>&1
