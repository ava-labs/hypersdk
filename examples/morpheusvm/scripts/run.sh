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

# build avalanchego
# https://github.com/ava-labs/avalanchego/releases
HYPERSDK_DIR=$HOME/.hypersdk

echo "working directory: $HYPERSDK_DIR"

AVALANCHEGO_PATH=${HYPERSDK_DIR}/avalanchego-${AVALANCHE_VERSION}/avalanchego
AVALANCHEGO_PLUGIN_DIR=${HYPERSDK_DIR}/avalanchego-${AVALANCHE_VERSION}/plugins

if [ ! -f "$AVALANCHEGO_PATH" ]; then
  echo "building avalanchego"
  CWD=$(pwd)

  # Clear old folders
  rm -rf "${HYPERSDK_DIR}"/avalanchego-"${AVALANCHE_VERSION}"
  mkdir -p "${HYPERSDK_DIR}"/avalanchego-"${AVALANCHE_VERSION}"
  rm -rf "${HYPERSDK_DIR}"/avalanchego-src
  mkdir -p "${HYPERSDK_DIR}"/avalanchego-src

  # Download src
  cd "${HYPERSDK_DIR}"/avalanchego-src
  git clone https://github.com/ava-labs/avalanchego.git
  cd avalanchego
  git checkout "${AVALANCHE_VERSION}"

  # Build avalanchego
  ./scripts/build.sh
  mv build/avalanchego "${HYPERSDK_DIR}"/avalanchego-"${AVALANCHE_VERSION}"

  cd "${CWD}"

  # Clear src
  rm -rf "${HYPERSDK_DIR}"/avalanchego-src
else
  echo "using previously built avalanchego"
fi

############################

echo "building morpheusvm"

# delete previous (if exists)
rm -f "${HYPERSDK_DIR}"/avalanchego-"${AVALANCHE_VERSION}"/plugins/qCNyZHrs3rZX458wPJXPJJypPf6w423A84jnfbdP2TPEmEE9u

# rebuild with latest code
go build \
-o "${HYPERSDK_DIR}"/avalanchego-"${AVALANCHE_VERSION}"/plugins/qCNyZHrs3rZX458wPJXPJJypPf6w423A84jnfbdP2TPEmEE9u \
./cmd/morpheusvm

############################
echo "building e2e.test"

prepare_ginkgo

ACK_GINKGO_RC=true ginkgo build ./tests/e2e
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
