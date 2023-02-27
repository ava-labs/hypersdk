#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -e

# Set the CGO flags to use the portable version of BLST
#
# We use "export" here instead of just setting a bash variable because we need
# to pass this flag to all child processes spawned by the shell.
export CGO_CFLAGS="-O -D__BLST_PORTABLE__"

if ! [[ "$0" =~ scripts/tests.integration.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

# to install the ginkgo binary (required for test build and run)
go install -v github.com/onsi/ginkgo/v2/ginkgo@v2.0.0-rc2 || true

# run with 3 embedded VMs
ACK_GINKGO_RC=true ginkgo \
run \
-v \
--fail-fast \
--cover \
--coverpkg github.com/ava-labs/hypersdk/examples/tokenvm/chain,github.com/ava-labs/hypersdk/examples/tokenvm/vm \
./tests/integration \
-- \
--vms 3 \
--min-price 1
