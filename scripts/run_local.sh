#!/bin/bash
#
# This script runs a complete set of benchmarks from the current
# workstation using its working copies of the repositories.
#
# This takes a while to run because it builds images, provisions VMs,
# and kicks off the benchmarks itself.
#
# The script run_cloud.sh starts a container that clones the repos and
# runs this script using HEAD from each client repo.
set -e

# By exporting TAG, benchmark.sh uses the same tag, which avoids
# repeating the same build.
export TAG
TAG=$(date "+%Y-%m-%d-%H-%M-%S")

# Give a title to this run
TITLE=${1}

# Source location
SCRIPTS=${GOPATH}/../scripts

function json() {
    python -c "import sys, json; print json.load(sys.stdin)$2" < $1
}

# TODO Update benchmark clients for: cpp ruby objc php ...
LANGUAGES="java nodejs" # nodejs golang python

NON_CONCURRENT="nodejs ruby"

# List of configurations
CONFIGS=`cd ${SCRIPTS}/config && ls -1 *.json | grep -v test`

function usage()
{
    echo "usage: ${0} title"
    echo "  GOPATH must be set"
    echo "  Configuration in \$GOPATH/../scripts"
}

function runtest()
{
    local language=${1}
    local config=${2}
    local conc=$(json ${SCRIPTS}/config/${config}.json '["Concurrency"]')

    if [ ${conc} != "1" ]; then
	if echo ${NON_CONCURRENT} | grep ${language} > /dev/null; then
	    echo "Skip concurrent ${language}/${config}"
	    return
	fi
    fi
    
    echo ./benchmark.sh ${TITLE} ${language} ${conc} ${config}
}

if [ "${TITLE}" = "" ]; then
    usage
    exit 1
fi

for language in ${LANGUAGES}; do
    for config in ${CONFIGS}; do
	name=`basename -s .json ${config}`
	runtest ${language} ${name}
    done
done
