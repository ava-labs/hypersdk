#!/usr/bin/env bash
# Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.


set -euo pipefail

if ! [[ "$0" =~ scripts/protobuf_codegen.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

## ensure the correct version of "buf" is installed
BUF_VERSION='1.40.1'
if [[ $(buf --version | cut -f2 -d' ') != "${BUF_VERSION}" ]]; then
  echo "could not find buf ${BUF_VERSION}, is it installed + in PATH?"
  exit 255
fi

# Ensure local binaries are in PATH
if [[ ! "$PATH" =~ $PWD/bin ]]; then
  export PATH=$PWD/bin:$PATH
fi

TARGET=$PWD/proto
if [ -n "${1:-}" ]; then
  TARGET="$1"
fi

cd "$TARGET"

echo "Running protobuf fmt..."
buf format -w

echo "Running protobuf lint check..."
if ! buf lint;  then
    echo "ERROR: protobuf linter failed"
    exit 1
fi

echo "Re-generating protobuf..."
if ! buf generate;  then
    echo "ERROR: protobuf generation failed"
    exit 1
fi
