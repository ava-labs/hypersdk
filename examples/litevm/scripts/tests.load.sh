#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -e

# Set the CGO flags to use the portable version of BLST
#
# We use "export" here instead of just setting a bash variable because we need
# to pass this flag to all child processes spawned by the shell.
export CGO_CFLAGS="-O -D__BLST_PORTABLE__"

if ! [[ "$0" =~ scripts/tests.load.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

# to install the ginkgo binary (required for test build and run)
go install -v github.com/onsi/ginkgo/v2/ginkgo@v2.0.0-rc2 || true

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
