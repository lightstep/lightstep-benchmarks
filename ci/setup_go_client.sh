#!/bin/bash

set -e

wget https://go.dev/dl/go1.17.8.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.17.8.linux-amd64.tar.gz

go get "github.com/lightstep/lightstep-tracer-go"
cd clients
go build go_client.go
