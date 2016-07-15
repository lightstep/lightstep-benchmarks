#!/bin/bash
set -e

# By exporting TAG, benchmark.sh uses the same tag, which avoids
# repeating the same build.
export TAG
TAG=$(date "+%Y-%m-%d-%H-%M-%S")

# Give a title to this run
TITLE=${1}

# Source location
SCRIPTS=${GOPATH}/../scripts

# This script uses 'jq'
JQ=jq

# TODO Update benchmark clients for: cpp ruby
LANGUAGES="golang python nodejs java"

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

    local conc=`${JQ} .Concurrency ${SCRIPTS}/config/${config}.json`
    
    ./benchmark.sh ${TITLE} ${language} ${conc} ${config}
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
