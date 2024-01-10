#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

HYPERSDK_PATH=$(
    cd "$(dirname "${BASH_SOURCE[0]}")"
    cd .. && pwd
)

source $HYPERSDK_PATH/scripts/common/utils.sh

set -e

check_repository_root scripts/tests.load.sh

# to install the ginkgo binary (required for test build and run)
go install -v github.com/onsi/ginkgo/v2/ginkgo@v2.0.0-rc2 || true

# alert the user if they do not have $GOPATH properly configured
check_command ginkgo
