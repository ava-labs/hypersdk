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

AGO_VERSION=v1.11.3
MAX_UINT64=18446744073709551615
MODE=${MODE:-run}
LOG_LEVEL=${LOG_LEVEL:-DEBUG}
AGO_LOG_LEVEL=${AGO_LOG_LEVEL:-INFO}
AGO_LOG_DISPLAY_LEVEL=${AGO_LOG_DISPLAY_LEVEL:-INFO}
EPOCH_DURATION=${EPOCH_DURATION:-10000}
VALIDITY_WINDOW=${VALIDITY_WINDOW:-9000}
MIN_BLOCK_GAP=${MIN_BLOCK_GAP:-1000}
UNLIMITED_USAGE=${UNLIMITED_USAGE:-false}
if [[ ${MODE} != "run" ]]; then
  LOG_LEVEL=debug
  AGO_LOG_DISPLAY_LEVEL=info
  MIN_BLOCK_GAP=250 #ms
  UNLIMITED_USAGE=true
fi

MAX_CHUNK_UNITS="1800000,15000,15000,2500,15000"
if ${UNLIMITED_USAGE}; then
  # If we don't limit the block size, AvalancheGo will reject the block.
  MAX_CHUNK_UNITS="1800000,${MAX_UINT64},${MAX_UINT64},${MAX_UINT64},${MAX_UINT64}"
fi

echo "Running with:"
echo AGO_LOG_LEVEL: "${AGO_LOG_LEVEL}"
echo AGO_LOG_DISPLAY_LEVEL: "${AGO_LOG_DISPLAY_LEVEL}"
echo AGO_VERSION: "${AGO_VERSION}"
echo MODE: "${MODE}"
echo LOG LEVEL: "${LOG_LEVEL}"
echo EPOCH_DURATION \(ms\): "${EPOCH_DURATION}"
echo VALIDITY_WINDOW \(ms\): "${VALIDITY_WINDOW}"
echo MIN_BLOCK_GAP \(ms\): "${MIN_BLOCK_GAP}"
echo MAX_CHUNK_UNITS: "${MAX_CHUNK_UNITS}"

############################
# build avalanchego
# https://github.com/ava-labs/avalanchego/releases
TMPDIR=/tmp/hypersdk

echo "working directory: $TMPDIR"

AVALANCHEGO_PATH=${TMPDIR}/avalanchego-${AGO_VERSION}/avalanchego
AVALANCHEGO_PLUGIN_DIR=${TMPDIR}/avalanchego-${AGO_VERSION}/plugins

if [ ! -f "$AVALANCHEGO_PATH" ]; then
  echo "building avalanchego"
  CWD=$(pwd)

  # Clear old folders
  rm -rf "${TMPDIR}"/avalanchego-"${AGO_VERSION}"
  mkdir -p "${TMPDIR}"/avalanchego-"${AGO_VERSION}"
  rm -rf "${TMPDIR}"/avalanchego-src
  mkdir -p "${TMPDIR}"/avalanchego-src

  # Download src
  cd "${TMPDIR}"/avalanchego-src
  git clone https://github.com/ava-labs/avalanchego.git
  cd avalanchego
  git checkout "${AGO_VERSION}"

  # Build avalanchego
  ./scripts/build.sh
  mv build/avalanchego "${TMPDIR}"/avalanchego-"${AGO_VERSION}"

  cd "${CWD}"
else
  echo "using previously built avalanchego"
fi

############################

############################
echo "building morpheusvm"

# delete previous (if exists)
rm -f "${TMPDIR}"/avalanchego-"${AGO_VERSION}"/plugins/qCNyZHrs3rZX458wPJXPJJypPf6w423A84jnfbdP2TPEmEE9u

# rebuild with latest code
go build \
-o "${TMPDIR}"/avalanchego-"${AGO_VERSION}"/plugins/qCNyZHrs3rZX458wPJXPJJypPf6w423A84jnfbdP2TPEmEE9u \
./cmd/morpheusvm

echo "building morpheus-cli"
go build -v -o "${TMPDIR}"/morpheus-cli ./cmd/morpheus-cli

# log everything in the avalanchego directory
find "${TMPDIR}"/avalanchego-"${AGO_VERSION}"

############################

############################

# Always create allocations (linter doesn't like tab)
#
# Addresses:
# morpheus1qrzvk4zlwj9zsacqgtufx7zvapd3quufqpxk5rsdd4633m4wz2fdjk97rwu: 323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7
# morpheus1qryyvfut6td0l2vwn8jwae0pmmev7eqxs2vw0fxpd2c4lr37jj7wvrj4vc3: ee11a050c75f0f47390f8ed98ab29fbce8c1f820b0245af56e1cb484a80c8022d77899baf0059747b8b685cfe62296f85f67083dc0bf8d2fab24c5ee3a7563b9
# morpheus1qp52zjc3ul85309xn9stldfpwkseuth5ytdluyl7c5mvsv7a4fc76g6c4w4: 34214e27f4c7d17315694968e37d999b848bb7b0bc95d679eb8163cf516c15dd9e77d9ebe639f9bece4260f4cce91ccf365dbce726da4299ff5a1b1ed31b339e
# morpheus1qzqjp943t0tudpw06jnvakdc0y8w790tzk7suc92aehjw0epvj93s0uzasn: ba09c65939a182f46879fcda172eabe9844d1f0a835a00c905dd2fa11b61a50ff38c9fdaef41e74730a732208284f2199fcd2f31779942662139884ca3f97a77
# morpheus1qz97wx3vl3upjuquvkulp56nk20l3jumm3y4yva7v6nlz5rf8ukty8fh27r: 3e5ab8a792187c8fa0a87e2171058d9a0c16ca07bc35c2cfb5e2132078fe18c0a70d00475d1e86ef32bb22397e47722c420dd4caf157400b83d9262af6bf0af5
echo "creating allocations file"
# Sum of allocations must be less than uint64 max
cat <<EOF > "${TMPDIR}"/allocations.json
[
  {"address":"morpheus1qrzvk4zlwj9zsacqgtufx7zvapd3quufqpxk5rsdd4633m4wz2fdjk97rwu", "balance":3000000000000000000},
  {"address":"morpheus1qryyvfut6td0l2vwn8jwae0pmmev7eqxs2vw0fxpd2c4lr37jj7wvrj4vc3", "balance":3000000000000000000},
  {"address":"morpheus1qp52zjc3ul85309xn9stldfpwkseuth5ytdluyl7c5mvsv7a4fc76g6c4w4", "balance":3000000000000000000},
  {"address":"morpheus1qzqjp943t0tudpw06jnvakdc0y8w790tzk7suc92aehjw0epvj93s0uzasn", "balance":3000000000000000000},
  {"address":"morpheus1qz97wx3vl3upjuquvkulp56nk20l3jumm3y4yva7v6nlz5rf8ukty8fh27r", "balance":3000000000000000000}
]
EOF

GENESIS_PATH=$2
if [[ -z "${GENESIS_PATH}" ]]; then
  echo "creating VM genesis file with allocations"
  rm -f "${TMPDIR}"/morpheusvm.genesis
  "${TMPDIR}"/morpheus-cli genesis generate "${TMPDIR}"/allocations.json \
  --epoch-duration "${EPOCH_DURATION}" \
  --validity-window "${VALIDITY_WINDOW}" \
  --max-chunk-units "${MAX_CHUNK_UNITS}" \
  --min-block-gap "${MIN_BLOCK_GAP}" \
  --genesis-file "${TMPDIR}"/morpheusvm.genesis
else
  echo "copying custom genesis file"
  rm -f "${TMPDIR}"/morpheusvm.genesis
  cp "${GENESIS_PATH}" "${TMPDIR}"/morpheusvm.genesis
fi

############################

############################

echo "creating vm config"
rm -f "${TMPDIR}"/morpheusvm.config
rm -rf "${TMPDIR}"/morpheusvm-e2e-profiles
cat <<EOF > "${TMPDIR}"/morpheusvm.config
{
  "chunkBuildFrequency": 333,
  "targetChunkBuildDuration": 250,
  "blockBuildFrequency": 100,
  "mempoolSize": 2147483648,
  "mempoolSponsorSize": 10000000,
  "authExecutionCores": 4,
  "precheckCores": 4,
  "actionExecutionCores": 2,
  "missingChunkFetchers": 48,
  "verifyAuth":true,
  "authRPCCores": 4,
  "authRPCBacklog": 10000000,
  "authGossipCores": 4,
  "authGossipBacklog": 10000000,
  "chunkStorageCores": 4,
  "chunkStorageBacklog": 10000000,
  "streamingBacklogSize": 10000000,
  "logLevel": "${LOG_LEVEL}",
  "continuousProfilerDir":"${TMPDIR}/morpheusvm-e2e-profiles/*"
}
EOF
mkdir -p "${TMPDIR}"/morpheusvm-e2e-profiles

############################

############################

echo "creating subnet config"
rm -f "${TMPDIR}"/morpheusvm.subnet
cat <<EOF > "${TMPDIR}"/morpheusvm.subnet
{
  "proposerMinBlockDelay": 0,
  "proposerNumHistoricalBlocks": 512
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
ANR_VERSION=e03e43a3610761e70694fee9e41611d30974a1a3
# version set
go install -v "${ANR_REPO_PATH}"@"${ANR_VERSION}"

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
--log-level=verbo \
--port=":12352" \
--grpc-gateway-port=":12353" &

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
--network-runner-log-level verbo \
--avalanchego-log-level "${AGO_LOG_LEVEL}" \
--avalanchego-log-display-level "${AGO_LOG_DISPLAY_LEVEL}" \
--network-runner-grpc-endpoint="0.0.0.0:12352" \
--network-runner-grpc-gateway-endpoint="0.0.0.0:12353" \
--avalanchego-path="${AVALANCHEGO_PATH}" \
--avalanchego-plugin-dir="${AVALANCHEGO_PLUGIN_DIR}" \
--vm-genesis-path="${TMPDIR}"/morpheusvm.genesis \
--vm-config-path="${TMPDIR}"/morpheusvm.config \
--subnet-config-path="${TMPDIR}"/morpheusvm.subnet \
--output-path="${TMPDIR}"/avalanchego-"${AGO_VERSION}"/output.yaml \
--mode="${MODE}"

############################
if [[ ${MODE} == "run" ]]; then
  echo "cluster is ready!"
  # We made it past initialization and should avoid shutting down the network
  KEEPALIVE=true
fi
