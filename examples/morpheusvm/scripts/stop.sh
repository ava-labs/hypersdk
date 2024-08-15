#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -e

# MORPHEUSVM_PATH=$(
#   cd "$(dirname "${BASH_SOURCE[0]}")"
#   cd .. && pwd
# )

WORK_DIR=${GOPATH}/src/github.com/ava-labs/hypersdk
MORPHEUSVM_PATH=${WORK_DIR}/examples/morpheusvm

ginkgo -v "$MORPHEUSVM_PATH"/tests/e2e/e2e.test -- --stop-network
