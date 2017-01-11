#!/bin/bash -e

(cd ${CLIENTS_DIR}/lightstep-tracer-ruby && rm pkg/lightstep-*.gem)
(cd ${CLIENTS_DIR}/lightstep-tracer-ruby && rake build && cp pkg/lightstep-*.gem ${DBUILD}/lightstep.gem)
(cd ${CLIENTS_DIR}/lightstep-tracer-ruby && cp benchmark.rb ${DBUILD})
