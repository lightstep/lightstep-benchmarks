#!/bin/sh

set -e

eval $(docker-machine env lightstep-benchmarks-cpu1)

cd $GOPATH

TMP=$GOPATH/build.$$
rm -rf $TMP
mkdir -p $TMP/node_modules

(cd $GOPATH/src && npm install 2>/dev/null)
(cd $GOPATH/src && GOOS=linux GOARCH=amd64 go build -o $TMP/controller controller.go)

tar -C src -cf - node_modules | (cd $TMP/node_modules && tar -xf -)
ln src/package.json $TMP
ln src/jsclient.js $TMP
ln ../docker/Dockerfile.nodejs $TMP/Dockerfile

docker build -t lightstep_nodejs_benchmark:latest $TMP
docker run -d lightstep_nodejs_benchmark:latest

rm -rf $TMP
