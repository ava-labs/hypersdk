#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -o errexit
set -o nounset
set -o pipefail

# Set the CGO flags to use the portable version of BLST
#
# We use "export" here instead of just setting a bash variable because we need
# to pass this flag to all child processes spawned by the shell.
export CGO_CFLAGS="-O -D__BLST_PORTABLE__"

if ! [[ "$0" =~ scripts/build.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

# Set default binary directory location
name="pkEmJQuTUic3dxzg8EYnktwn4W7uCHofNcwiYo458vodAUbY7"

# Build morpheusvm, which is run as a subprocess
mkdir -p ./build

echo "Building morpheusvm in ./build/$name"
go build -o ./build/$name ./cmd/morpheusvm

echo "Building morpheus-cli in ./build/morpheus-cli"
go build -o ./build/morpheus-cli ./cmd/morpheus-cli
