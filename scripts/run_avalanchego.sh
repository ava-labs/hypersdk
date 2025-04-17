#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -euo pipefail

REPO_ROOT=$(cd "$( dirname "${BASH_SOURCE[0]}" )"; cd .. && pwd )

# Set AVALANCHE_VERSION and ensure CGO is configured
. "${REPO_ROOT}"/scripts/constants.sh

# Need to set the binary name or the name will be 'main'
GOLANG_BINARY_NAME=avalanchego \
  "${REPO_ROOT}"/scripts/run_versioned_binary.sh github.com/ava-labs/avalanchego/main "${AVALANCHE_VERSION}" "${@}"
