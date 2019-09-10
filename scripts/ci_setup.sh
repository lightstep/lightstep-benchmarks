#!/bin/bash

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

sudo apt-get update 
sudo apt-get install --no-install-recommends --no-install-suggests -y \
  trickle

source "${SCRIPT_DIR}/install_proto.sh"
source "${SCRIPT_DIR}/generate_proto.sh"
source "${SCRIPT_DIR}/pip_install.sh"
