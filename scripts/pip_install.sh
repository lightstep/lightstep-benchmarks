#!/bin/bash

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
PROJECT_DIR="${SCRIPT_DIR}/.."

cd ${PROJECT_DIR}

pip install --upgrade pip==19.1.* setuptools==41.0.*
pip install -r requirements.txt
