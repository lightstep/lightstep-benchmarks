#!/bin/bash
#
# usage:
#   benchmark.sh <title> <client-lang> <num-cpus> <config-name>
#
# e.g., test named jvm_9_0_8 of java client, 4 cpus, using
# 'four-cpus-1kb-logs' config:
#
#   benchmark.sh jvm_9_0_8 java 4 four-cpus-1kb-logs
#
# pass <num-cpus>="local" to run locally
#
# e.g. local java client using 'test' config:
#   benchmark.sh jvm_9_0_8 java local test

set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# arguments
TITLE=${1}
CLIENT=${2}
CPUS=${3}
TEST_CONFIG_NAME=${4}

# TAG may be set in run_local.sh
if [ -z "${TAG}" ]; then
    TAG=$(date "+%Y-%m-%d-%H-%M-%S")
fi

# local debugging option, set CPUs="local"
LOCAL=no
case ${CPUS} in
    local|LOCAL)
	LOCAL=yes
	PROJECT_ID=local
	;;
esac

# naming constants
BUCKET="lightstep-client-benchmarks"
CLOUD_ZONE="us-central1-a"
CLOUD_MACH_BASE="n1-standard-"
IMG_BASE="gcr.io/${PROJECT_ID}/bench-${CLIENT}"
VM="bench-${TITLE}-${CLIENT}-${CPUS}-${TEST_CONFIG_NAME}"
TEST_PARAMS_NAME="${BENCHMARK_PARAMS}"
if [ -z "${TEST_PARAMS_NAME}" ]; then
    TEST_PARAMS_NAME=fast
fi

# file system paths
SCRIPTS_DIR="${DIR}"
CLIENTS_DIR="$(cd ${DIR}/../clients && pwd)"
DBUILD="${DIR}/../build/build.$$"
TEST_CONFIG="${SCRIPTS_DIR}/config/${TEST_CONFIG_NAME}.json"
TEST_PARAMS="${SCRIPTS_DIR}/params/${TEST_PARAMS_NAME}.json"

export SCRIPTS_DIR
export CLIENTS_DIR

# gcp constants
SCOPED="https://www.googleapis.com/auth"

# command-line tools
GCLOUD="gcloud"
LDOCKER="docker"
GLDOCKER="${GCLOUD} docker --"

SSH="${GCLOUD} compute ssh --project ${PROJECT_ID} --zone ${CLOUD_ZONE}"
SSH_TO="${SSH} ${VM} --"         # ssh to the VM
GBASH="${SSH_TO} sudo bash"      # root shell on the VM
GDOCKER="${SSH_TO} sudo docker"  # docker on the VM

# use the developer gcloud profile, restore it on exit
GCLOUD_CONFIG="devel"

if [ "${LOCAL}" = "no" -a -z "${PROJECT_ID}" ]; then
    echo Please set PROJECT_ID
    exit 1
fi

function usage()
{
    echo "usage: $0 title client cpus config"
    echo "  GOPATH must be set"
    echo "  Configuration in \${SCRIPTS_DIR}"
}

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

function on_exit()
{
    rm -rf "${DBUILD}"
}

function build()
{
    rm -rf ${DBUILD}
    mkdir -p ${DBUILD}

    (GOOS=linux GOARCH=amd64 go build -o ${DBUILD}/controller ${GOPATH}/src/github.com/lightstep/lightstep-benchmarks/cmd/controller/controller.go)

    . ${SCRIPTS_DIR}/docker/${CLIENT}.sh

    ln ${SCRIPTS_DIR}/docker/Dockerfile.${CLIENT} ${DBUILD}/Dockerfile

    ${LDOCKER} build -t ${IMG_BASE}:${TAG} ${DBUILD}
    ${LDOCKER} tag ${IMG_BASE}:${TAG} ${IMG_BASE}:latest
    echo Built!
}

function launch_on_gcp()
{
    if [ "${LOCAL}" = "no" ]; then
	echo Push ${IMG_BASE}:${TAG}
	${GLDOCKER} push ${IMG_BASE}:${TAG}
	echo Push ${IMG_BASE}:latest
	${GLDOCKER} push ${IMG_BASE}:latest
    fi
    if ${SSH_TO} true 2> /dev/null; then
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
	    if ${SSH_TO} true; then
		break
	    fi
	    sleep 2
	done
    fi

    # Reset the VM.
    ${GBASH} <<EOF
${LDOCKER} ps -q -all | xargs docker kill 2> /dev/null
${LDOCKER} ps -q -all | xargs docker rm  2> /dev/null
${GLDOCKER} pull ${IMG_BASE}:latest
mkdir -p /tmp/config
chmod 0777 /tmp/config
EOF

    # Place the configuration
    ${GCLOUD} compute copy-files ${TEST_CONFIG} ${VM}:/tmp/config/config.json
    ${GCLOUD} compute copy-files ${TEST_PARAMS} ${VM}:/tmp/config/params.json

    # Daemonize
    ${GDOCKER} run -d \
	       -v /tmp/config:/tmp/config \
	       -e BENCHMARK_CONFIG_NAME=${TEST_CONFIG_NAME} \
	       -e BENCHMARK_CONFIG_FILE=/tmp/config/config.json \
	       -e BENCHMARK_PARAMS_FILE=/tmp/config/params.json \
	       -e BENCHMARK_TITLE=${TITLE} \
	       -e BENCHMARK_BUCKET=${BUCKET} \
	       -e BENCHMARK_ZONE=${CLOUD_ZONE} \
	       -e BENCHMARK_PROJECT=${PROJECT_ID} \
	       -e BENCHMARK_INSTANCE=${VM} \
	       -e BENCHMARK_CLIENT=${CLIENT} \
	       -e BENCHMARK_VERBOSE=${BENCHMARK_VERBOSE} \
	       --name ${VM} \
	       ${IMG_BASE}:latest \
	       ./controller

    # Note: the controller deletes its own VM
}

function run_local()
{
    ${LDOCKER} kill ${VM} || true 2> /dev/null
    ${LDOCKER} rm ${VM} || true 2> /dev/null
    ${LDOCKER} run \
	       -v ${SCRIPTS_DIR}:/tmp/scripts \
	       -e BENCHMARK_CONFIG_NAME=${TEST_CONFIG_NAME} \
	       -e BENCHMARK_CONFIG_FILE=/tmp/scripts/config/${TEST_CONFIG_NAME}.json \
	       -e BENCHMARK_PARAMS_FILE=/tmp/scripts/params/${TEST_PARAMS_NAME}.json \
	       -e BENCHMARK_TITLE=${TITLE} \
	       -e BENCHMARK_BUCKET=${BUCKET} \
	       -e BENCHMARK_CLIENT=${CLIENT} \
	       -e BENCHMARK_VERBOSE=${BENCHMARK_VERBOSE} \
	       --name ${VM} \
	       ${IMG_BASE}:latest \
	       ./controller
    
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

if [ ! -d "${SCRIPTS_DIR}" ]; then
    echo "Scripts directory not found (${SCRIPTS_DIR})"
    exit 1
fi

trap on_exit EXIT

if [ "${LOCAL}" != "yes" ]; then
    set_config
fi

build

if [ "${LOCAL}" = "yes" ]; then
    run_local
else
    launch_on_gcp
fi

