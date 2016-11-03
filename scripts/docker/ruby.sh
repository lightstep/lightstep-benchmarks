#!/bin/bash

# TODO rbclient moves to /github.com/lightstep/lightstep-tracer-ruby/benchmark

ln ${GOPATH}/src/rbclient.rb ${DBUILD}/rbclient.rb
echo "source 'https://rubygems.org'" > ${DBUILD}/Gemfile
echo "gem 'lightstep'" >> ${DBUILD}/Gemfile

