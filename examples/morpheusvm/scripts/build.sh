#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

source ../../scripts/common/build.sh
source ../../scripts/constants.sh

set_cgo_flags

# Construct the correct path to morpheusvm directory
MORPHEUSVM_PATH=$(
    cd "$(dirname "${BASH_SOURCE[0]}")"
    cd .. && pwd
)

build_project "$MORPHEUSVM_PATH" "morpheusvm" "pkEmJQuTUic3dxzg8EYnktwn4W7uCHofNcwiYo458vodAUbY7" 