#!/bin/bash

# TODO goclient moves to /github.com/lightstep/lightstep-tracer-go/benchmark

(GOOS=linux GOARCH=amd64 go build -o ${DBUILD}/goclient ${GOPATH}/src/github.com/lightstep/lightstep-benchmarks/goclient.go)
