#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -euo pipefail

# Ensure the go command is run from the root of the repository
REPO_ROOT=$(cd "$( dirname "${BASH_SOURCE[0]}" )"; cd .. && pwd )
cd "${REPO_ROOT}"

# Set AVALANCHE_VERSION
. ./scripts/constants.sh

go run github.com/ava-labs/avalanchego/tests/fixture/tmpnet/tmpnetctl@"${AVALANCHE_VERSION}" "${@}"
