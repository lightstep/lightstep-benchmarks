#!/bin/sh

set -e

JAVA=${GOPATH}/src/github.com/lightstep/lightstep-tracer-java
JRE=${JAVA}/lightstep-tracer-jre
VERSION=`cat ${JAVA}/common/VERSION`

(cd ${JRE} && ./gradlew -b build.gradle jar)
(cd ${JRE} && ./gradlew -b benchmark.gradle jar)

ln ${JRE}/build/libs/lightstep-benchmark-${VERSION}.jar ${DBUILD}/lightstep-benchmark.jar
