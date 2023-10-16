#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -e

# Set the CGO flags to use the portable version of BLST
#
# We use "export" here instead of just setting a bash variable because we need
# to pass this flag to all child processes spawned by the shell.
export CGO_CFLAGS="-O -D__BLST_PORTABLE__"

# to run E2E tests (terminates cluster afterwards)
# MODE=test ./scripts/run.sh
if ! [[ "$0" =~ scripts/run.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

VERSION=v1.10.12
MAX_UINT64=18446744073709551615
MODE=${MODE:-run}
LOGLEVEL=${LOGLEVEL:-info}
STATESYNC_DELAY=${STATESYNC_DELAY:-0}
MIN_BLOCK_GAP=${MIN_BLOCK_GAP:-100}
STORE_TXS=${STORE_TXS:-false}
UNLIMITED_USAGE=${UNLIMITED_USAGE:-false}
if [[ ${MODE} != "run" && ${MODE} != "run-single" ]]; then
  LOGLEVEL=debug
  STATESYNC_DELAY=100000000 # 100ms
  MIN_BLOCK_GAP=250 #ms
  STORE_TXS=true
  UNLIMITED_USAGE=true
fi

WINDOW_TARGET_UNITS="40000000,450000,450000,450000,450000"
MAX_BLOCK_UNITS="1800000,15000,15000,2500,15000"
if ${UNLIMITED_USAGE}; then
  WINDOW_TARGET_UNITS="${MAX_UINT64},${MAX_UINT64},${MAX_UINT64},${MAX_UINT64},${MAX_UINT64}"
  # If we don't limit the block size, AvalancheGo will reject the block.
  MAX_BLOCK_UNITS="1800000,${MAX_UINT64},${MAX_UINT64},${MAX_UINT64},${MAX_UINT64}"
fi

echo "Running with:"
echo LOGLEVEL: ${LOGLEVEL}
echo VERSION: ${VERSION}
echo MODE: ${MODE}
echo STATESYNC_DELAY \(ns\): ${STATESYNC_DELAY}
echo MIN_BLOCK_GAP \(ms\): ${MIN_BLOCK_GAP}
echo STORE_TXS: ${STORE_TXS}
echo WINDOW_TARGET_UNITS: ${WINDOW_TARGET_UNITS}
echo MAX_BLOCK_UNITS: ${MAX_BLOCK_UNITS}

############################
# build avalanchego
# https://github.com/ava-labs/avalanchego/releases
GOARCH=$(go env GOARCH)
GOOS=$(go env GOOS)
TMPDIR=/tmp/hypersdk

echo "working directory: $TMPDIR"

AVALANCHEGO_PATH=${TMPDIR}/avalanchego-${VERSION}/avalanchego
AVALANCHEGO_PLUGIN_DIR=${TMPDIR}/avalanchego-${VERSION}/plugins

if [ ! -f "$AVALANCHEGO_PATH" ]; then
  echo "building avalanchego"
  CWD=$(pwd)

  # Clear old folders
  rm -rf ${TMPDIR}/avalanchego-${VERSION}
  mkdir -p ${TMPDIR}/avalanchego-${VERSION}
  rm -rf ${TMPDIR}/avalanchego-src
  mkdir -p ${TMPDIR}/avalanchego-src

  # Download src
  cd ${TMPDIR}/avalanchego-src
  git clone https://github.com/ava-labs/avalanchego.git
  cd avalanchego
  git checkout ${VERSION}

  # Build avalanchego
  ./scripts/build.sh
  mv build/avalanchego ${TMPDIR}/avalanchego-${VERSION}

  cd ${CWD}
else
  echo "using previously built avalanchego"
fi

############################

############################
echo "building tokenvm"

# delete previous (if exists)
rm -f ${TMPDIR}/avalanchego-${VERSION}/plugins/tHBYNu8ikqo4MWMHehC9iKB9mR5tB3DWzbkYmTfe9buWQ5GZ8

# rebuild with latest code
go build \
-o ${TMPDIR}/avalanchego-${VERSION}/plugins/tHBYNu8ikqo4MWMHehC9iKB9mR5tB3DWzbkYmTfe9buWQ5GZ8 \
./cmd/tokenvm

echo "building token-cli"
go build -v -o ${TMPDIR}/token-cli ./cmd/token-cli

# log everything in the avalanchego directory
find ${TMPDIR}/avalanchego-${VERSION}

############################

############################

# Always create allocations (linter doesn't like tab)
#
# Make sure to replace this address with your own address
# if you are starting your own devnet (otherwise anyone can access
# funds using the included demo.pk)
echo "creating allocations file"
cat <<EOF > ${TMPDIR}/allocations.json
[{"address":"token1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97nsjzf3yp", "balance":10000000000000000000}]
EOF

GENESIS_PATH=$2
if [[ -z "${GENESIS_PATH}" ]]; then
  echo "creating VM genesis file with allocations"
  rm -f ${TMPDIR}/tokenvm.genesis
  ${TMPDIR}/token-cli genesis generate ${TMPDIR}/allocations.json \
  --window-target-units ${WINDOW_TARGET_UNITS} \
  --max-block-units ${MAX_BLOCK_UNITS} \
  --min-block-gap ${MIN_BLOCK_GAP} \
  --genesis-file ${TMPDIR}/tokenvm.genesis
else
  echo "copying custom genesis file"
  rm -f ${TMPDIR}/tokenvm.genesis
  cp ${GENESIS_PATH} ${TMPDIR}/tokenvm.genesis
fi

############################

############################

# When running a validator, the [trackedPairs] should be empty/limited or
# else malicious entities can attempt to stuff memory with dust orders to cause
# an OOM.
echo "creating vm config"
rm -f ${TMPDIR}/tokenvm.config
rm -rf ${TMPDIR}/tokenvm-e2e-profiles
cat <<EOF > ${TMPDIR}/tokenvm.config
{
  "mempoolSize": 10000000,
  "mempoolPayerSize": 10000000,
  "mempoolExemptPayers":["token1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97nsjzf3yp"],
  "signatureVerificationCores": 2,
  "rootGenerationCores": 2,
  "transactionExecutionCores": 2,
  "verifySignatures": true,
  "storeTransactions": ${STORE_TXS},
  "streamingBacklogSize": 10000000,
  "trackedPairs":["*"],
  "logLevel": "${LOGLEVEL}",
  "continuousProfilerDir":"${TMPDIR}/tokenvm-e2e-profiles/*",
  "stateSyncServerDelay": ${STATESYNC_DELAY}
}
EOF
mkdir -p ${TMPDIR}/tokenvm-e2e-profiles

############################

############################

echo "creating subnet config"
rm -f ${TMPDIR}/tokenvm.subnet
cat <<EOF > ${TMPDIR}/tokenvm.subnet
{
  "proposerMinBlockDelay": 0,
  "proposerNumHistoricalBlocks": 50000
}
EOF

############################

############################
echo "building e2e.test"
# to install the ginkgo binary (required for test build and run)
go install -v github.com/onsi/ginkgo/v2/ginkgo@v2.1.4

# alert the user if they do not have $GOPATH properly configured
if ! command -v ginkgo &> /dev/null
then
    echo -e "\033[0;31myour golang environment is misconfigued...please ensure the golang bin folder is in your PATH\033[0m"
    echo -e "\033[0;31myou can set this for the current terminal session by running \"export PATH=\$PATH:\$(go env GOPATH)/bin\"\033[0m"
    exit
fi

ACK_GINKGO_RC=true ginkgo build ./tests/e2e
./tests/e2e/e2e.test --help

#################################
# download avalanche-network-runner
# https://github.com/ava-labs/avalanche-network-runner
ANR_REPO_PATH=github.com/ava-labs/avalanche-network-runner
ANR_VERSION=v1.7.2
# version set
go install -v ${ANR_REPO_PATH}@${ANR_VERSION}

#################################
# run "avalanche-network-runner" server
GOPATH=$(go env GOPATH)
if [[ -z ${GOBIN+x} ]]; then
  # no gobin set
  BIN=${GOPATH}/bin/avalanche-network-runner
else
  # gobin set
  BIN=${GOBIN}/avalanche-network-runner
fi

killall avalanche-network-runner || true

echo "launch avalanche-network-runner in the background"
$BIN server \
--log-level verbo \
--port=":12352" \
--grpc-gateway-port=":12353" &
PID=${!}

############################
# By default, it runs all e2e test cases!
# Use "--ginkgo.skip" to skip tests.
# Use "--ginkgo.focus" to select tests.

KEEPALIVE=false
function cleanup() {
  if [[ ${KEEPALIVE} = true ]]; then
    echo "avalanche-network-runner is running in the background..."
    echo ""
    echo "use the following command to terminate:"
    echo ""
    echo "./scripts/stop.sh;"
    echo ""
    exit
  fi

  echo "avalanche-network-runner shutting down..."
  ./scripts/stop.sh;
}
trap cleanup EXIT

echo "running e2e tests"
./tests/e2e/e2e.test \
--ginkgo.v \
--network-runner-log-level verbo \
--avalanchego-log-level ${LOGLEVEL} \
--network-runner-grpc-endpoint="0.0.0.0:12352" \
--network-runner-grpc-gateway-endpoint="0.0.0.0:12353" \
--avalanchego-path=${AVALANCHEGO_PATH} \
--avalanchego-plugin-dir=${AVALANCHEGO_PLUGIN_DIR} \
--vm-genesis-path=${TMPDIR}/tokenvm.genesis \
--vm-config-path=${TMPDIR}/tokenvm.config \
--subnet-config-path=${TMPDIR}/tokenvm.subnet \
--output-path=${TMPDIR}/avalanchego-${VERSION}/output.yaml \
--mode=${MODE}

############################
if [[ ${MODE} == "run" || ${MODE} == "run-single" ]]; then
  echo "cluster is ready!"
  # We made it past initialization and should avoid shutting down the network
  KEEPALIVE=true
fi
