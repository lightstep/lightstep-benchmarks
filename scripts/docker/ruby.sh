#!/bin/bash

# TODO rbclient moves to lightstep-tracer-ruby

ln ${GOPATH}/src/github.com/lightstep/lightstep-benchmarks/rbclient.rb ${DBUILD}/rbclient.rb
echo "source 'https://rubygems.org'" > ${DBUILD}/Gemfile
echo "gem 'lightstep'" >> ${DBUILD}/Gemfile

