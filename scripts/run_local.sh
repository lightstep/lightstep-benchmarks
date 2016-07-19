#!/bin/bash
#
# This script runs a complete set of benchmarks from the current
# workstation using its working copies of the repositories.
#
# This takes a while to run because it builds images, provisions VMs,
# and kicks off the benchmarks itself.
#
# TODO write run_cloud.sh to starts polyglot container that clones the
# repos, installs build tools, and runs this script using HEAD from
# each client repo.

# By exporting TAG, benchmark.sh uses the same tag, which avoids
# repeating the same build.
export TAG
TAG=$(date "+%Y-%m-%d-%H-%M-%S")

# "test" or "logs"
MODE=${1}

# Give a title to this run
TITLE=${2}

# Source location
SCRIPTS=${GOPATH}/../scripts

case ${MODE} in
    test)
	CMD="./benchmark.sh"
	set -e
	;;
    logs)
	CMD="./show_logs.sh"
	;;
    *)
	echo "Invalid mode ${MODE}, should be 'test' or 'logs'."
	exit 1
esac
     

function json() {
    python -c "import sys, json; print json.load(sys.stdin)$2" < $1
}

if [ -z "${LANGUAGES}" ]; then
    # TODO Update benchmark clients for: cpp objc php ...
    LANGUAGES="java nodejs ruby" # golang python
fi

# Languages that do not support concurrent testing
NON_CONCURRENT="nodejs ruby"

# List of configurations
if [ -z "${CONFIGS}" ]; then
    CONFIGS=`cd ${SCRIPTS}/config && ls -1 *.json | grep -v test`
fi

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

    echo ======================================================================
    echo ${CMD} ${TITLE} ${language} ${conc} ${config}
    ${CMD} ${TITLE} ${language} ${conc} ${config}
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
