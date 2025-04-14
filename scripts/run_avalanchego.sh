#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -euo pipefail

REPO_ROOT=$(cd "$( dirname "${BASH_SOURCE[0]}" )"; cd .. && pwd )

. "${REPO_ROOT}"/scripts/install_versioned_binary.sh
install_versioned_binary avalanchego github.com/ava-labs/avalanchego/main

"${REPO_ROOT}"/build/avalanchego "${@}"
