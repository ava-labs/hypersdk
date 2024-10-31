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
# Construct the correct path to testvm directory
TEST_VM_PATH=$(
  cd "$(dirname "${BASH_SOURCE[0]}")"
  cd .. && pwd
)

build_project "$TEST_VM_PATH" "testvm" "pkEmJQuTUic3dxzg8EYnktwn4W7uCHofNcwiYo458vodAUbY7"
