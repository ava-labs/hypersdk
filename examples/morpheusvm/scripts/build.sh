#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -o errexit
set -o nounset
set -o pipefail

# Get the directory of the script, even if sourced from another directory
SCRIPT_DIR=$(cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd)

# shellcheck source=/scripts/common/build.sh
source "$SCRIPT_DIR"/../../../scripts/common/build.sh
# shellcheck source=/scripts/constants.sh
source "$SCRIPT_DIR"/../../../scripts/constants.sh
# Construct the correct path to morpheusvm directory
MORPHEUSVM_PATH=$(
  cd "$(dirname "${BASH_SOURCE[0]}")"
  cd .. && pwd
)

build_project "$MORPHEUSVM_PATH" "morpheusvm" "pkEmJQuTUic3dxzg8EYnktwn4W7uCHofNcwiYo458vodAUbY7"

# also build the morpheusvm cli(moved from old build script)
CLI_PATH=$MORPHEUSVM_PATH/build/morpheus-cli
echo "Building morpheus-cli in $CLI_PATH"
mkdir -p "$(dirname "$CLI_PATH")"
go build -o "$CLI_PATH" ./cmd/morpheus-cli
