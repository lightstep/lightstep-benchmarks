FROM golang:latest
MAINTAINER team@lightstep.com

WORKDIR /

RUN apt-get update
RUN apt-get install -y git-core
RUN curl -sL https://deb.nodesource.com/setup_5.x | bash -
RUN apt-get install -y nodejs
RUN apt-get install -y python-pip

RUN git clone https://github.com/lightstep/lightstep-benchmarks.git

ENV GOPATH=/lightstep-benchmarks/go

WORKDIR ${GOPATH}/src

RUN npm install
RUN pip install lightstep
RUN go get -d

RUN go build goclient.go
RUN go build controller.go
