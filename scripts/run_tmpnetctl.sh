#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -euo pipefail

REPO_ROOT=$(cd "$( dirname "${BASH_SOURCE[0]}" )"; cd .. && pwd )

# Set AVALANCHE_VERSION and ensure CGO is configured
. "${REPO_ROOT}"/scripts/constants.sh

. "${REPO_ROOT}"/scripts/install_versioned_binary.sh
install_versioned_binary "${REPO_ROOT}" tmpnetctl github.com/ava-labs/avalanchego/tests/fixture/tmpnet/tmpnetctl "${AVALANCHE_VERSION}"

"${REPO_ROOT}"/build/tmpnetctl "${@}"
