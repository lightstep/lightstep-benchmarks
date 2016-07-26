#!/bin/bash

JOBNAME="ls_cpp_builder"
IMAGE="${JOBNAME}:latest"

CBUILD="${GOPATH}/build.cpp.$$"

MOUNT1="${GOPATH}/src/github.com/lightstep/lightstep-tracer-cpp:/data/lightstep-tracer-cpp"
MOUNT2="/tmp/lightstep_downloads:/tmp/lightstep_downloads"

export OPENSSL_ROOT_DIR=/usr/local/Cellar/openssl/1.0.2h_1

function on_exit()
{
    rm -rf "${CBUILD}"
}

function build()
{
    rm -rf ${CBUILD}
    mkdir ${CBUILD}

    ln ${GOPATH}/../scripts/docker/Dockerfile.build_cpp ${CBUILD}/Dockerfile

    docker kill ${JOBNAME} || true
    docker rm ${JOBNAME} || true
    docker build -t ${IMAGE} ${CBUILD}
    docker run -v ${MOUNT1} -v ${MOUNT2} --name ${JOBNAME} ${IMAGE} /bin/sh -c "(cd /data/lightstep-tracer-cpp && rm -rf linux && mkdir -p linux/bin && ./bootstrap.sh && sh configure --prefix=/data/lightstep-tracer-cpp/linux && make && make install && cp ./test/c++11/cppclient ./linux/bin)"
}

trap on_exit EXIT

build

cp ${GOPATH}/src/github.com/lightstep/lightstep-tracer-cpp/linux/bin/cppclient ${DBUILD}
cp ${GOPATH}/src/github.com/lightstep/lightstep-tracer-cpp/linux/lib/* ${DBUILD}
