#!/bin/bash

# TODO jsclient moves to lightstep-tracer-javascript

(cd ${GOPATH}/src/github.com/lightstep/lightstep-benchmarks && npm install 2>/dev/null)

mkdir -p ${DBUILD}/node_modules

# TODO the -L here is undesireable b/c it copies 4x Docker context,
# however this allows use of "npm link" local development.
(cd ${GOPATH}/src/github.com/lightstep && \
 tar -L -C lightstep-benchmarks -cf - node_modules) | \
    (cd ${DBUILD}/node_modules && tar -xf -)
ln ${GOPATH}/src/github.com/lightstep/lightstep-benchmarks/package.json ${DBUILD}
ln ${GOPATH}/src/github.com/lightstep/lightstep-benchmarks/jsclient.js ${DBUILD}
