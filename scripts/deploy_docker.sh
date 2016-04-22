#!/bin/sh

DATETIME=$(date "+%Y-%m-%d_%H-%M-%S")
BASENAME=lightstep-benchmark
PROJECT_ID=$(gcloud config --format text list project | cut -d' ' -f2)

NAME=gcr.io/${PROJECT_ID}/${BASENAME}

docker build --no-cache -t ${NAME}:${DATETIME} ${GOPATH}/../docker
docker tag -f ${NAME}:${DATETIME} ${NAME}:latest

gcloud docker push ${NAME}:${DATETIME}
gcloud docker push ${NAME}:latest

# Some docker-machine magic to create a machine
eval $(docker-machine env lightstep-benchmarks-1cpu)

gcloud docker run -- -d ${NAME}:${DATETIME} ./controller --logtostderr
