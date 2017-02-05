#!/bin/bash

JOBNAME="ls_cpp_builder"
IMAGE="${JOBNAME}:latest"

CBUILD="${GOPATH}/build.cpp.$$"
BENROOT="${GOPATH}/src/github.com/lightstep/lightstep-benchmarks"
CPPROOT="${BENROOT}/clients/lightstep-tracer-cpp"

MOUNT1="${CPPROOT}:/data/lightstep-tracer-cpp"
MOUNT2="/tmp/lightstep_downloads:/tmp/lightstep_downloads"

function on_exit()
{
    rm -rf "${CBUILD}"
}

function build()
{
    rm -rf ${CBUILD}
    mkdir ${CBUILD}

    ln ${BENROOT}/scripts/docker/Dockerfile.build_cpp ${CBUILD}/Dockerfile

    docker kill ${JOBNAME} || true
    docker rm ${JOBNAME} || true
    docker build -t ${IMAGE} ${CBUILD}
    docker run -v ${MOUNT1} -v ${MOUNT2} --name ${JOBNAME} ${IMAGE} /bin/sh -c "(cd /data/lightstep-tracer-cpp && rm -rf linux && mkdir -p linux/bin && ./bootstrap.sh && sh configure --prefix=/data/lightstep-tracer-cpp/linux && make && make install && cp ./test/c++11/cppclient ./linux/bin && cp /usr/local/lib/libproto*.so.* /usr/local/lib/libgrpc*.so.* ./linux/lib)"
    # TODO: copying duplicate files into the context, should use tar for the .so.* pattern above
}

trap on_exit EXIT

build

cp ${CPPROOT}/linux/bin/cppclient ${DBUILD}
cp ${CPPROOT}/linux/lib/* ${DBUILD}
