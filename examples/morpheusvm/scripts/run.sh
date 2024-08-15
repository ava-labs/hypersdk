#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -e

# to run E2E tests (terminates cluster afterwards)
# MODE=test ./scripts/run.sh
MODE=${MODE:-run}
# if ! [[ "$0" =~ scripts/run.sh ]]; then
#   echo "must be run from morpheusvm root"
#   exit 255
# fi

WORK_DIR=${GOPATH}/src/github.com/ava-labs/hypersdk

# shellcheck source=/scripts/constants.sh
# source ../../scripts/constants.sh
# shellcheck source=/scripts/common/utils.sh
source ${WORK_DIR}/scripts/common/utils.sh

MORPHEUSVM_DIR=${WORK_DIR}/examples/morpheusvm
VERSION=d729e5c7ef9f008c3e89cd7131148ad3acda2e34

############################
# build avalanchego
# https://github.com/ava-labs/avalanchego/releases
TMPDIR=/tmp/hypersdk

echo "working directory: $TMPDIR"

AVALANCHEGO_PATH=${TMPDIR}/avalanchego-${VERSION}/avalanchego
AVALANCHEGO_PLUGIN_DIR=${TMPDIR}/avalanchego-${VERSION}/plugins

if [ ! -f "$AVALANCHEGO_PATH" ]; then
  echo "building avalanchego"
  CWD=$(pwd)

  # Clear old folders
  rm -rf "${TMPDIR}"/avalanchego-"${VERSION}"
  mkdir -p "${TMPDIR}"/avalanchego-"${VERSION}"
  rm -rf "${TMPDIR}"/avalanchego-src
  mkdir -p "${TMPDIR}"/avalanchego-src

  # Download src
  cd "${TMPDIR}"/avalanchego-src
  git clone https://github.com/ava-labs/avalanchego.git
  cd avalanchego
  git checkout "${VERSION}"

  # Build avalanchego
  ./scripts/build.sh
  mv build/avalanchego "${TMPDIR}"/avalanchego-"${VERSION}"

  cd "${CWD}"
else
  echo "using previously built avalanchego"
fi

############################

echo "building morpheusvm"

# delete previous (if exists)
rm -f "${TMPDIR}"/avalanchego-"${VERSION}"/plugins/qCNyZHrs3rZX458wPJXPJJypPf6w423A84jnfbdP2TPEmEE9u

# rebuild with latest code
go build \
-o "${TMPDIR}"/avalanchego-"${VERSION}"/plugins/qCNyZHrs3rZX458wPJXPJJypPf6w423A84jnfbdP2TPEmEE9u \
${MORPHEUSVM_DIR}/cmd/morpheusvm

############################
echo "building e2e.test"

prepare_ginkgo

ACK_GINKGO_RC=true ginkgo build ${MORPHEUSVM_DIR}/tests/e2e
${MORPHEUSVM_DIR}/tests/e2e/e2e.test --help

additional_args=("$@")

if [[ ${MODE} == "run" ]]; then
  echo "applying ginkgo.focus=Ping and --reuse-network to setup local network"
  additional_args+=("--ginkgo.focus=Ping")
  additional_args+=("--reuse-network")
fi

echo "running e2e tests"
${MORPHEUSVM_DIR}/tests/e2e/e2e.test \
--ginkgo.v \
--avalanchego-path="${AVALANCHEGO_PATH}" \
--plugin-dir="${AVALANCHEGO_PLUGIN_DIR}" \
"${additional_args[@]}"
