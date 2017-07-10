#!/bin/sh

set -e

JAVA=${CLIENTS_DIR}/lightstep-tracer-java
BENCHMARK=${JAVA}/benchmark

cd ${JAVA}
make build
VERSION=`mvn org.apache.maven.plugins:maven-help-plugin:2.1.1:evaluate -Dexpression=project.version | grep -v '\['`

ln ${BENCHMARK}/target/benchmark-${VERSION}.jar ${DBUILD}/lightstep-benchmark.jar
