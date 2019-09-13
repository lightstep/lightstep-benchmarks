#!/bin/bash

set -e

[ -z "${OPENTRACING_VERSION}" ] && export OPENTRACING_VERSION="v1.5.0"
[ -z "${LIGHTSTEP_VERSION}" ] && export LIGHTSTEP_VERSION="v0.11.0"
[ -z "${PROTOBUF_VERSION}" ] && export PROTOBUF_VERSION="3.5.1"

apt-get update 
apt-get install --no-install-recommends --no-install-suggests -y \
                build-essential \
                libgflags-dev \
                libevent-dev \
                curl

mkdir -p /build
pushd /build

# Build protobuf
curl -OL https://github.com/google/protobuf/releases/download/v${PROTOBUF_VERSION}/protobuf-cpp-${PROTOBUF_VERSION}.tar.gz
tar zxf protobuf-cpp-${PROTOBUF_VERSION}.tar.gz
cd protobuf-${PROTOBUF_VERSION}
./configure
make && make install
ldconfig
cd /build

# Build OpenTracing
git clone -b ${OPENTRACING_VERSION} https://github.com/opentracing/opentracing-cpp.git
cd opentracing-cpp
mkdir .build && cd .build
cmake -DBUILD_TESTING=OFF -DBUILD_SHARED_LIBS=OFF ..
make && make install
cd /build

# Build lightstep
git clone -b ${LIGHTSTEP_VERSION} https://github.com/lightstep/lightstep-tracer-cpp.git
cd lightstep-tracer-cpp
mkdir .build
cd .build
cmake -DCMAKE_BUILD_TYPE=RELEASE \
      -DBUILD_SHARED_LIBS=OFF \
      -DBUILD_TESTING=OFF \
      -DWITH_DYNAMIC_LOAD=OFF \
      -DWITH_GRPC=OFF \
      -DWITH_LIBEVENT=ON \
      -DWITH_CARES=OFF \
      ..
make && make install
cd /build

popd
