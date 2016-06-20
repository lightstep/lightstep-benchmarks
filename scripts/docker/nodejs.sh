#!/bin/bash

# TODO jsclient moves to /github.com/lightstep/lightstep-tracer-javascript/benchmark
(cd ${GOPATH}/src && npm install 2>/dev/null)

mkdir -p ${DBUILD}/node_modules

# TODO the -L here is undesireable b/c it copies 4x Docker context,
# however this allows use of "npm link" local development.
(cd ${GOPATH} && tar -L -C src -cf - node_modules) | \
    (cd ${DBUILD}/node_modules && tar -xf -)
ln ${GOPATH}/src/package.json ${DBUILD}
ln ${GOPATH}/src/jsclient.js ${DBUILD}
