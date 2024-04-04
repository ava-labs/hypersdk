#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -e

if ! [[ "$0" =~ scripts/tests.load.sh ]]; then
  echo "must be run from morpheusvm root"
  exit 255
fi

# shellcheck source=/scripts/common/utils.sh
source ../../scripts/common/utils.sh

prepare_ginkgo

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
--trace="${TRACE}"
