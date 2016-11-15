#!/bin/bash

# TODO pyclient moves to /github.com/lightstep/lightstep-tracer-python/benchmark

ln ${GOPATH}/src/github.com/lightstep/lightstep-benchmarks/pyclient.py ${DBUILD}/pyclient.py 
ln ${GOPATH}/src/github.com/lightstep/lightstep-benchmarks/requirements.txt ${DBUILD}/requirements.txt
