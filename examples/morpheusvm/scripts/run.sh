#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -e

# to run E2E tests (terminates cluster afterwards)
# MODE=test ./scripts/run.sh
MODE=${MODE:-run}

# Ensure the morpheusvm root as the working directory
cd "$(dirname "${BASH_SOURCE[0]}")" && cd ..

# shellcheck source=/scripts/constants.sh
source ../../scripts/constants.sh
# shellcheck source=/scripts/common/utils.sh
source ../../scripts/common/utils.sh

# Ensure absolute paths to avoid dependency on the working directory
AVALANCHEGO_PATH="$(realpath "${AVALANCHEGO_PATH:-../../scripts/run_avalanchego.sh}")"
AVALANCHEGO_PLUGIN_DIR="${AVALANCHEGO_PLUGIN_DIR:-../../build/plugins}"
mkdir -p "${AVALANCHEGO_PLUGIN_DIR}"
AVALANCHEGO_PLUGIN_DIR=$(realpath "${AVALANCHEGO_PLUGIN_DIR}")

echo "testing build of avalanchego"
${AVALANCHEGO_PATH} --version

echo "building morpheusvm"

# rebuild with latest code
go build -o "${AVALANCHEGO_PLUGIN_DIR}"/qCNyZHrs3rZX458wPJXPJJypPf6w423A84jnfbdP2TPEmEE9u ./cmd/morpheusvm

echo "building e2e.test"

ACK_GINKGO_RC=true ../../bin/ginkgo build ./tests/e2e
./tests/e2e/e2e.test --help

args=(
  --ginkgo.v
  --avalanchego-path="${AVALANCHEGO_PATH}"
  --plugin-dir="${AVALANCHEGO_PLUGIN_DIR}"
)

if [[ ${MODE} == "run" ]]; then
  echo "applying ginkgo.focus=Ping and --reuse-network to setup local network"
  args+=("--ginkgo.focus=Ping")
  args+=("--reuse-network")
fi

# Append any additional arguments passed by the user
args+=("$@")

echo "running e2e tests"
./tests/e2e/e2e.test "${args[@]}"
