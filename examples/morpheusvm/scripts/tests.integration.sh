#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -e

if [ -n "$1" ]; then
    # Attempt to change to the directory provided as the argument
    if cd "$1"; then
        echo "cd $1"
    else
        echo "failed to cd to directory; $1"
        exit 255
    fi
fi

# shellcheck source=/scripts/common/utils.sh
source ../../scripts/common/utils.sh
# shellcheck source=/scripts/constants.sh
source ../../scripts/constants.sh

rm_previous_cov_reports
prepare_ginkgo

# run with 3 embedded VMs
ACK_GINKGO_RC=true ginkgo \
run \
-v \
--fail-fast \
-cover \
-covermode=atomic \
-coverpkg=github.com/ava-labs/hypersdk/... \
-coverprofile=integration.coverage.out \
./tests/integration \
--vms 3

# output generate coverage html
go tool cover -html=integration.coverage.out -o=integration.coverage.html
