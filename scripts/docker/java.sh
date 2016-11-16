#!/bin/sh

set -e

JAVA=${CLIENTS_DIR}/lightstep-tracer-java
JRE=${JAVA}/lightstep-tracer-jre
VERSION=`awk 'BEGIN { FS = "=" }; { printf("%s", $2) }' ${JAVA}/gradle.properties`

(cd ${JAVA} && ./gradlew -b build.gradle build)

ln ${JRE}/build/libs/lightstep-benchmark-${VERSION}.jar ${DBUILD}/lightstep-benchmark.jar
ln ${JRE}/build/libs/lightstep-tracer-jre-${VERSION}.jar ${DBUILD}/lightstep-tracer-jre.jar
