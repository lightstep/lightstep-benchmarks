#!/bin/bash

SRC_FOLDER="proto"
DST_FOLDER="generated"

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
SATELLITE_DIR="$SCRIPT_DIR/../satellite"

# begin by installing Protobuf
curl -OL https://github.com/google/protobuf/releases/download/v3.2.0/protoc-3.2.0-linux-x86_64.zip
unzip protoc-3.2.0-linux-x86_64.zip -d protoc3
sudo mv protoc3/bin/* /usr/local/bin/
sudo mv protoc3/include/* /usr/local/include/

# now compile proto files --> python files
mkdir $SATELLITE_DIR/generated
protoc -I=$SATELLITE_DIR/$SRC_FOLDER --python_out=$SATELLITE_DIR/$DST_FOLDER $SATELLITE_DIR/$SRC_FOLDER/collector.proto
