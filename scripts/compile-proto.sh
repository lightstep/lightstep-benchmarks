#!/bin/bash

SRC_DIR="../proto"
DST_DIR="../generated"
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

protoc -I=$SCRIPT_DIR/$SRC_DIR --python_out=$SCRIPT_DIR/$DST_DIR $SCRIPT_DIR/$SRC_DIR/collector.proto
