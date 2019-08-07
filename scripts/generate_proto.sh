#!/bin/bash

SRC_FOLDER="proto"
DST_FOLDER="generated"

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
SATELLITE_DIR="$SCRIPT_DIR/../satellite"

# now compile proto files --> python files
mkdir $SATELLITE_DIR/generated
protoc -I=$SATELLITE_DIR/$SRC_FOLDER --python_out=$SATELLITE_DIR/$DST_FOLDER $SATELLITE_DIR/$SRC_FOLDER/collector.proto
