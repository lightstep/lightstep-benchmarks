#!/bin/bash

IMAGE="ls_cpp_builder:latest"

CBUILD="${GOPATH}/build.cpp.$$"

MOUNT="${GOPATH}/src/github.com/lightstep/lightstep-tracer-cpp:/data/lightstep-tracer-cpp"

function on_exit()
{
    rm -rf "${CBUILD}"
}

function build()
{
    rm -rf ${CBUILD}
    mkdir ${CBUILD}

    ln ${GOPATH}/../scripts/docker/Dockerfile.build_cpp ${CBUILD}/Dockerfile

    docker build -t ${IMAGE} ${CBUILD}
    docker run -v ${MOUNT} ${IMAGE} /bin/sh -c "(cd /data/lightstep-tracer-cpp && rm -rf linux && mkdir -p linux/bin && ./bootstrap.sh && sh configure --prefix=/data/lightstep-tracer-cpp/linux && make && make install && cp ./test/c++11/cppclient ./linux/bin)"
}

trap on_exit EXIT

build

cp ${GOPATH}/src/github.com/lightstep/lightstep-tracer-cpp/linux/bin/cppclient ${DBUILD}
cp ${GOPATH}/src/github.com/lightstep/lightstep-tracer-cpp/linux/lib/* ${DBUILD}
