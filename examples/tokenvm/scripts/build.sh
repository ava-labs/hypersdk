#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

source ../../scripts/common/build.sh
source ../../scripts/constants.sh

set_cgo_flags

# Construct the correct path to tokenvm directory
TOKENVM_PATH=$(
    cd "$(dirname "${BASH_SOURCE[0]}")"
    cd .. && pwd
)

build_project "$TOKENVM_PATH" "tokenvm" tHBYNu8ikqo4MWMHehC9iKB9mR5tB3DWzbkYmTfe9buWQ5GZ8
build_project "$TOKENVM_PATH" "token-cli"
build_project "$TOKENVM_PATH" "token-faucet"
build_project "$TOKENVM_PATH" "token-feed"