#!/bin/bash

(GOOS=linux GOARCH=amd64 go build -o ${DBUILD}/goclient ${GOPATH}/src/goclient.go)
