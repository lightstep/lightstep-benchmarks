#!/bin/bash

set -e

apt-get update 
apt-get install --no-install-recommends --no-install-suggests -y \
                golang

go get "github.com/lightstep/lightstep-tracer-go"
cd clients
go build go_client.go
