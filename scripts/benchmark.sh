#!/bin/bash

set -e

TITLE=${1}
CLIENT=${2}
CPUS=${3}
TEST_CONFIG_BASE=${4}

if [ -z "${TAG}" ]; then
    TAG=$(date "+%Y-%m-%d-%H-%M-%S")
fi

BUCKET="lightstep-client-benchmarks"
GCLOUD_CONFIG="devel"
CLOUD_ZONE="us-central1-a"
PROJECT_ID="lightstep-dev"
CLOUD_MACH_BASE="n1-standard-"
IMG_BASE="gcr.io/${PROJECT_ID}/bench-${CLIENT}"
VM="bench-${TITLE}-${CLIENT}-${CPUS}-${TEST_CONFIG_BASE}"

DBUILD="${GOPATH}/build.$$"
SCRIPTS="${GOPATH}/../scripts"
TEST_CONFIG="${SCRIPTS}/config/${TEST_CONFIG_BASE}.json"
SCOPED="https://www.googleapis.com/auth"

GCLOUD="gcloud"
SSH="${GCLOUD} compute ssh --project ${PROJECT_ID} --zone ${CLOUD_ZONE}"
GDOCKER="${SSH} ${VM} -- sudo docker"
GBASH="${SSH} ${VM} -- sudo bash"
LDOCKER="docker"
GLDOCKER="${GCLOUD} ${LDOCKER}"

STANDING_GCLOUD_CONFIG=`${GCLOUD} config configurations list | grep True | awk '{print $1}'`

function usage()
{
    echo "usage: $0 title client cpus config"
    echo "  GOPATH must be set"
    echo "  Configuration in \$GOPATH/../scripts"
}

function set_config()
{
    ${GCLOUD} config configurations activate ${GCLOUD_CONFIG} 2> /dev/null
}

function build()
{
    HAVE_IT=`${GLDOCKER} images -- -q ${IMG_BASE}:${TAG}`
    if [ ! -z "${HAVE_IT}" ]; then
	echo "${IMG_BASE}:${TAG} was already built"
	return
    fi

    rm -rf ${DBUILD}
    mkdir ${DBUILD}

    (GOOS=linux GOARCH=amd64 go build -o ${DBUILD}/controller $GOPATH/src/controller.go)

    . ${SCRIPTS}/docker/${CLIENT}.sh

    ln ${SCRIPTS}/docker/Dockerfile.${CLIENT} ${DBUILD}/Dockerfile
    ln ${TEST_CONFIG} ${DBUILD}/config.json

    ${LDOCKER} build -t ${IMG_BASE}:${TAG} ${DBUILD}
    ${LDOCKER} tag ${IMG_BASE}:${TAG} ${IMG_BASE}:latest
    ${GCLOUD} docker push ${IMG_BASE}:${TAG}
    ${GCLOUD} docker push ${IMG_BASE}:latest
}

function on_exit()
{
    rm -rf "${DBUILD}"
    ${GCLOUD} config configurations activate ${STANDING_GCLOUD_CONFIG} 2> /dev/null
}

function dockerize()
{
    if ${SSH} ${VM} -- true 2> /dev/null; then
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

	# Wait for sshd to come up
	while true; do
	    if ${SSH} ${VM} -- true; then
		break
	    fi
	    sleep 2
	done
    fi

    ${GBASH} <<EOF
${LDOCKER} ps -q -all | xargs docker kill 2> /dev/null
${LDOCKER} ps -q -all | xargs docker rm  2> /dev/null
${GLDOCKER} pull ${IMG_BASE}:latest
${LDOCKER} run \
	   -d \
	   -e BENCHMARK_CONFIG=${TEST_CONFIG_BASE} \
	   -e BENCHMARK_TITLE=${TITLE} \
	   -e BENCHMARK_BUCKET=${BUCKET} \
	   -e BENCHMARK_ZONE=${CLOUD_ZONE} \
	   -e BENCHMARK_PROJECT=${PROJECT_ID} \
	   -e BENCHMARK_INSTANCE=${VM} \
	   --name ${VM} \
	   ${IMG_BASE}:latest
EOF

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
