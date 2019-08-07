#!/bin/bash

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
PROJECT_DIR="${SCRIPT_DIR}/.."

cd ${PROJECT_DIR}

sudo pip install --upgrade pip==19.1.* setuptools==41.0.*
sudo pip install -r requirements.txt
