#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -e

# to run E2E tests (terminates cluster afterwards)
# MODE=test ./scripts/run.sh
MODE=${MODE:-run}
if ! [[ "$0" =~ scripts/run.sh ]]; then
  echo "must be run from hyperevm root"
  exit 255
fi

# shellcheck source=/scripts/constants.sh
source ../../scripts/constants.sh
# shellcheck source=/scripts/common/utils.sh
source ../../scripts/common/utils.sh

AVALANCHE_VERSION="6dea1b366756"

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

echo "building hyperevm"

# delete previous (if exists)
rm -f "${HYPERSDK_DIR}"/avalanchego-"${AVALANCHE_VERSION}"/plugins/o1f99fVdFpnvKTT9gM3dC11Du2t9cNQhRuuDzdF6urYaPbG4M

# rebuild with latest code
go build \
-o "${HYPERSDK_DIR}"/avalanchego-"${AVALANCHE_VERSION}"/plugins/o1f99fVdFpnvKTT9gM3dC11Du2t9cNQhRuuDzdF6urYaPbG4M \
./cmd/hyperevm

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
