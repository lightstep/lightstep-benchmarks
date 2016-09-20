FROM java:latest

WORKDIR /data

RUN apt-get update && \
    apt-get install -qqy \
	ca-certificates

COPY controller /data
COPY lightstep-benchmark.jar /data
COPY lightstep-tracer-jre.jar /data

ENV CLASSPATH /data/lightstep-tracer-jre.jar:/data/lightstep-benchmark.jar
