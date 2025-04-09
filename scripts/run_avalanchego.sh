#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -euo pipefail

# This script runs the avalanchego command with `go run` to ensure
# that the version used always matches the vendored version of
# avalanchego. It does induce a latency penalty since there is always
# a check for whether things need to be built.

# Ensure the go command is run from the root of the repository
REPO_ROOT=$(cd "$( dirname "${BASH_SOURCE[0]}" )"; cd .. && pwd )
cd "${REPO_ROOT}"

# Set AVALANCHE_VERSION and ensure CGO is configured
. ./scripts/constants.sh

go run github.com/ava-labs/avalanchego/main@"${AVALANCHE_VERSION}" "${@}"
