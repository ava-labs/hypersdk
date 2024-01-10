#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

HYPERSDK_PATH=$(
    cd "$(dirname "${BASH_SOURCE[0]}")"
    cd .. && pwd
)
source $HYPERSDK_PATH/common/utils.sh

function rm_previous_cov_reports() {
    rm -f integration.coverage.out
    rm -f integration.coverage.html
}

function prepare_ginkgo() {
    set -e

    check_repository_root scripts/tests.integration.sh

    # to install the ginkgo binary (required for test build and run)
    go install -v github.com/onsi/ginkgo/v2/ginkgo@v2.0.0-rc2 || true
    
    # alert the user if they do not have $GOPATH properly configured
    check_command ginkgo
}