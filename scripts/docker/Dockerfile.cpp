FROM ubuntu:16.04

WORKDIR /data

RUN apt-get update && \
    apt-get install -qqy \
	ca-certificates \
	libboost-all-dev \
	libcppnetlib-dev \
	libssl-dev

ENV LD_LIBRARY_PATH=/data

COPY * /data/
