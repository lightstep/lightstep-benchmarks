#!/bin/bash -e

# Note: clients/lightstep-tracer-go/cmd/benchmark/vendor copies
# ../../benchlib (i.e., the server-side code).

(cd ${CLIENTS_DIR} && GOOS=linux GOARCH=amd64 go build -o ${DBUILD}/goclient lightstep-tracer-go/cmd/benchmark/main.go)
