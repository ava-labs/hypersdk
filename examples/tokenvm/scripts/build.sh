#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -o errexit
set -o nounset
set -o pipefail

# Get the directory of the script, even if sourced from another directory
SCRIPT_DIR=$(
  cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd
)

source "$SCRIPT_DIR"/../../../scripts/common/build.sh
source "$SCRIPT_DIR"/../../../scripts/constants.sh

# Construct the correct path to tokenvm directory
TOKENVM_PATH=$(
  cd "$(dirname "${BASH_SOURCE[0]}")"
  cd .. && pwd
)

build_project "$TOKENVM_PATH" "tokenvm" tHBYNu8ikqo4MWMHehC9iKB9mR5tB3DWzbkYmTfe9buWQ5GZ8
build_project "$TOKENVM_PATH" "token-cli"
build_project "$TOKENVM_PATH" "token-faucet"
build_project "$TOKENVM_PATH" "token-feed"
