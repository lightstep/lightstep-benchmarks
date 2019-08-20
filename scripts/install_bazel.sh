#!/bin/bash

# NEED TO RUN THIS SCRIPT WITH SUDO

# this version is compatible with lightstep-cpp-tracer
BAZEL_VERSION="0.27.0"

apt-get install --no-install-recommends --no-install-suggests -y wget \
wget https://github.com/bazelbuild/bazel/releases/download/${BAZEL_VERSION}/bazel-${BAZEL_VERSION}-installer-linux-x86_64.sh
chmod +x bazel-${BAZEL_VERSION}-installer-linux-x86_64.sh
./bazel-${BAZEL_VERSION}-installer-linux-x86_64.sh
