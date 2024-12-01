#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -o errexit
set -o nounset
set -o pipefail

# Get the directory of the script, even if sourced from another directory
SCRIPT_DIR=$(cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd)

# shellcheck source=/scripts/constants.sh
source "$SCRIPT_DIR"/../../../scripts/constants.sh
# Construct the correct path to morpheusvm directory
MORPHEUSVM_PATH=$(
  cd "$(dirname "${BASH_SOURCE[0]}")"
  cd .. && pwd
)

if [[ $# -eq 1 ]]; then
    BINARY_PATH=$1
elif [[ $# -eq 0 ]]; then
    # Set default binary directory location
    name="pkEmJQuTUic3dxzg8EYnktwn4W7uCHofNcwiYo458vodAUbY7"
    BINARY_PATH=$MORPHEUSVM_PATH/build/$name
else
    echo "Invalid arguments to build morpheusvm. Requires zero (default location) or one argument to specify binary location."
    exit 1
fi

cd "$MORPHEUSVM_PATH"
mkdir -p "$(dirname "$BINARY_PATH")"
go build -o "$BINARY_PATH" ./cmd/morpheusvm