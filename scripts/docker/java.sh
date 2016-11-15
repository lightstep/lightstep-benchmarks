#!/bin/sh

set -e

JAVA=${CLIENTS_DIR}/lightstep-tracer-java
JRE=${JAVA}/lightstep-tracer-jre
VERSION=`cat ${JAVA}/common/VERSION`

(cd ${JRE} && ./gradlew -b build.gradle jar)
(cd ${JRE} && ./gradlew -b benchmark.gradle jar)

ln ${JRE}/build/libs/lightstep-benchmark-${VERSION}.jar ${DBUILD}/lightstep-benchmark.jar
ln ${JRE}/build/libs/lightstep-tracer-jre-${VERSION}.jar ${DBUILD}/lightstep-tracer-jre.jar
