#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

HYPERSDK_PATH=$(
    cd "$(dirname "${BASH_SOURCE[0]}")"
    cd .. && pwd
)

source $HYPERSDK_PATH/scripts/common/utils.sh

set -e

HYPERSDK_PATH=$(
  cd "$(dirname "${BASH_SOURCE[0]}")"
  cd .. && pwd
)
source "$HYPERSDK_PATH"/scripts/constants.sh

set_cgo_flags

check_repository_root scripts/tests.unit.sh

go test -race -timeout="10m" -coverprofile="coverage.out" -covermode="atomic" $(go list ./... | grep -v tests)
