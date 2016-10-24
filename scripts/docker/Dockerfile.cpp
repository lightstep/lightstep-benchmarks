FROM ubuntu:16.04

WORKDIR /data

RUN apt-get update && \
    apt-get install -qqy \
	ca-certificates

ENV LD_LIBRARY_PATH=/data

COPY * /data/
