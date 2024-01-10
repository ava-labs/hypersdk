#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

source ../../scripts/tests.load.sh

# run with 5 embedded VMs
TRACE=${TRACE:-false}
echo "tracing enabled=${TRACE}"
ACK_GINKGO_RC=true ginkgo \
run \
-v \
--fail-fast \
./tests/load \
-- \
--dist "uniform" \
--vms 5 \
--accts 10000 \
--txs 500000 \
--trace=${TRACE}
