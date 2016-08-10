#!/bin/bash
#
# usage:
#   benchmark.sh <title> <client-lang> <num-cpus> <config-name>
#
# e.g.:
#   benchmark.sh jvm_9_0_8 java 4 four-cpus-1kb-logs

# TODO Add safety here to avoid losing long experiments!

set -e

# arguments
TITLE=${1}
CLIENT=${2}
CPUS=${3}
TEST_CONFIG_NAME=${4}

# TAG may be set in run_local.sh
if [ -z "${TAG}" ]; then
    TAG=$(date "+%Y-%m-%d-%H-%M-%S")
fi

# naming constants
BUCKET="lightstep-client-benchmarks"
CLOUD_ZONE="us-central1-a"
PROJECT_ID="lightstep-dev"
CLOUD_MACH_BASE="n1-standard-"
IMG_BASE="gcr.io/${PROJECT_ID}/bench-${CLIENT}"
VM="bench-${TITLE}-${CLIENT}-${CPUS}-${TEST_CONFIG_NAME}"

# file system paths
DBUILD="${GOPATH}/build.$$"
SCRIPTS="${GOPATH}/../scripts"
TEST_CONFIG="${SCRIPTS}/config/${TEST_CONFIG_NAME}.json"

# gcp constants
SCOPED="https://www.googleapis.com/auth"

# command-line tools
GCLOUD="gcloud"
LDOCKER="docker"
GLDOCKER="${GCLOUD} docker"

SSH="${GCLOUD} compute ssh --project ${PROJECT_ID} --zone ${CLOUD_ZONE}"
SSH_TO="${SSH} ${VM} --"         # ssh to the VM
GBASH="${SSH_TO} sudo bash"      # root shell on the VM
GDOCKER="${SSH_TO} sudo docker"  # docker on the VM

# use the developer gcloud profile, restore it on exit
GCLOUD_CONFIG="devel"
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

function on_exit()
{
    rm -rf "${DBUILD}"
    ${GCLOUD} config configurations activate ${STANDING_GCLOUD_CONFIG} 2> /dev/null
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

    ${LDOCKER} build -t ${IMG_BASE}:${TAG} ${DBUILD}
    ${LDOCKER} tag ${IMG_BASE}:${TAG} ${IMG_BASE}:latest
    echo Push ${IMG_BASE}:${TAG}
    ${GCLOUD} docker push ${IMG_BASE}:${TAG}
    echo Push ${IMG_BASE}:latest
    ${GCLOUD} docker push ${IMG_BASE}:latest
    echo Built!
}

function dockerize()
{
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

    # Daemonize
    ${GDOCKER} run -d \
	       -v /tmp/config:/tmp/config \
	       -e BENCHMARK_CONFIG_NAME=${TEST_CONFIG_NAME} \
	       -e BENCHMARK_CONFIG_FILE=/tmp/config/config.json \
	       -e BENCHMARK_TITLE=${TITLE} \
	       -e BENCHMARK_BUCKET=${BUCKET} \
	       -e BENCHMARK_ZONE=${CLOUD_ZONE} \
	       -e BENCHMARK_PROJECT=${PROJECT_ID} \
	       -e BENCHMARK_INSTANCE=${VM} \
	       -e BENCHMARK_CLIENT=${CLIENT} \
	       --name ${VM} \
	       ${IMG_BASE}:latest \
	       ./controller --logtostderr -v=1

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
build
dockerize
