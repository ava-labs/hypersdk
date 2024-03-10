#!/usr/bin/env bash
# Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -e

# Check that avalanche-cli is installed
if ! [ -x "$(command -v avalanche)" ]; then
  echo 'avalanche is not installed' >&2
  exit 1
fi

# Set the CGO flags to use the portable version of BLST
#
# We use "export" here instead of just setting a bash variable because we need
# to pass this flag to all child processes spawned by the shell.
export CGO_CFLAGS="-O -D__BLST_PORTABLE__"

# Ensure that the script is being run from the repository root
if ! [[ "$0" =~ scripts/deploy.devnet.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

# Create temporary directory for the deployment
TMPDIR=/tmp/morpheusvm-deploy
rm -rf $TMPDIR && mkdir -p $TMPDIR
echo "working directory: $TMPDIR"

# Build morpheus-cli
echo "building morpheus-cli"
go build -v -o "${TMPDIR}"/morpheus-cli ./cmd/morpheus-cli

# Generate genesis file and configs
ADDRESS=morpheus1qrzvk4zlwj9zsacqgtufx7zvapd3quufqpxk5rsdd4633m4wz2fdjk97rwu
MIN_BLOCK_GAP=1000
MIN_UNIT_PRICE="1,1,1,1,1"
MAX_CHUNK_UNITS="1800000,15000,15000,15000,15000"
echo "creating allocations file"
cat <<EOF > "${TMPDIR}"/allocations.json
[
  {"address":"${ADDRESS}", "balance":10000000000000000000}
]
EOF

"${TMPDIR}"/morpheus-cli genesis generate "${TMPDIR}"/allocations.json \
--min-unit-price "${MIN_UNIT_PRICE}" \
--max-chunk-units "${MAX_CHUNK_UNITS}" \
--min-block-gap "${MIN_BLOCK_GAP}" \
--genesis-file "${TMPDIR}"/morpheusvm.genesis

cat <<EOF > "${TMPDIR}"/morpheusvm.config
{
  "mempoolSize": 10000000,
  "mempoolSponsorSize": 10000000,
  "mempoolExemptSponsors":["${ADDRESS}"],
  "authVerificationCores": 10,
  "transactionExecutionCores": 4,
  "rootGenerationCores": 14,
  "verifyAuth":true,
  "storeTransactions": false,
  "streamingBacklogSize": 10000000,
  "logLevel": "INFO"
}
EOF

cat <<EOF > "${TMPDIR}"/morpheusvm.subnet
{
  "proposerMinBlockDelay": 0,
  "proposerNumHistoricalBlocks": 512
}
EOF

cat <<EOF > "${TMPDIR}"/node.config
{
  "log-level":"%s",
  "log-display-level":"%s",
  "proposervm-use-current-height":true,
  "throttler-inbound-validator-alloc-size":"10737418240",
  "throttler-inbound-at-large-alloc-size":"10737418240",
  "throttler-inbound-node-max-processing-msgs":"100000",
  "throttler-inbound-bandwidth-refill-rate":"1073741824",
  "throttler-inbound-bandwidth-max-burst-size":"1073741824",
  "throttler-inbound-cpu-validator-alloc":"100000",
  "throttler-inbound-disk-validator-alloc":"10737418240000",
  "throttler-outbound-validator-alloc-size":"10737418240",
  "throttler-outbound-at-large-alloc-size":"10737418240",
  "consensus-on-accept-gossip-validator-size":"10",
  "consensus-on-accept-gossip-peer-size":"10",
  "network-compression-type":"zstd",
  "consensus-app-concurrency":"16",
  "profile-continuous-enabled":true,
  "profile-continuous-freq":"1m",
  "http-host":"",
  "http-allowed-origins": "*",
  "http-allowed-hosts": "*"
}
EOF

avalanche node devnet wiz vryxTest1 pkEmJQuTUic3dxzg8EYnktwn4W7uCHofNcwiYo458vodAUbY7 --num-apis 1,1 --num-validators 2,2 --region us-east-1,us-east-2 --aws --node-type c5.4xlarge --separate-monitoring-instance --default-validator-params --custom-vm-repo-url="https://www.github.com/ava-labs/hypersdk/" --custom-vm-branch deploy-devnet --custom-vm-build-script="examples/morpheusvm/scripts/build.sh" --custom-subnet=true --subnet-genesis="${TMPDIR}/morpheusvm.genesis" --subnet-config="${TMPDIR}/morpheusvm.genesis" --chain-config="${TMPDIR}/morpheusvm.config" --node-config="${TMPDIR}/node.config"
