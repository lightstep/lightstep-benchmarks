#!/bin/bash

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

source "${SCRIPT_DIR}/install_proto.sh"
source "${SCRIPT_DIR}/generate_proto.sh"
source "${SCRIPT_DIR}/pip_install.sh"
