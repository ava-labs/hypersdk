#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

# Cleanup from previous runs
rm -rf /tmp/avalanche-ops
mkdir /tmp/avalanche-ops


# Set constants
export ARCH_TYPE=$(uname -m)
[ $ARCH_TYPE = x86_64 ] && ARCH_TYPE=amd64
echo ARCH_TYPE: ${ARCH_TYPE}
export OS_TYPE=$(uname | tr '[:upper:]' '[:lower:]')
echo OS_TYPE: ${OS_TYPE}
export AVALANCHEGO_VERSION="1.10.11"
echo AVALANCHEGO_VERSION: ${AVALANCHEGO_VERSION}
export HYPERSDK_VERSION="0.0.14"
echo HYPERSDK_VERSION: ${HYPERSDK_VERSION}
# TODO: set deploy os/arch

# Check valid setup
if [ ${OS_TYPE} != 'darwin' ]; then
  echo 'os is not supported' >&2
  exit 1
fi
if [ ${ARCH_TYPE} != 'arm64' ]; then
  echo 'arch is not supported' >&2
  exit 1
fi
if ! [ -x "$(command -v aws)" ]; then
  echo 'aws-cli is not installed' >&2
  exit 1
fi
if ! [ -x "$(command -v yq)" ]; then
  echo 'yq is not installed' >&2
  exit 1
fi

# Install avalanche-ops
echo 'installing avalanche-ops...'
rm -f /tmp/avalancheup-aws
wget https://github.com/ava-labs/avalanche-ops/releases/download/latest/avalancheup-aws.aarch64-apple-darwin
mv ./avalancheup-aws.aarch64-apple-darwin /tmp/avalancheup-aws
chmod +x /tmp/avalancheup-aws
/tmp/avalancheup-aws --help

# Install token-cli
echo 'installing token-cli...'
rm -f /tmp/avalanche-ops/token-cli
mkdir -p /tmp/avalanche-ops
wget "https://github.com/ava-labs/hypersdk/releases/download/v${HYPERSDK_VERSION}/tokenvm_${HYPERSDK_VERSION}_${OS_TYPE}_${ARCH_TYPE}.tar.gz"
mkdir tmp-hypersdk
tar -xvf tokenvm_${HYPERSDK_VERSION}_${OS_TYPE}_${ARCH_TYPE}.tar.gz -C tmp-hypersdk
rm -rf tokenvm_${HYPERSDK_VERSION}_${OS_TYPE}_${ARCH_TYPE}.tar.gz
mv tmp-hypersdk/token-cli /tmp/avalanche-ops/token-cli
rm -rf tmp-hypersdk

# Download tokenvm
echo 'downloading tokenvm...'
rm -f /tmp/avalanche-ops/tokenvm
wget "https://github.com/ava-labs/hypersdk/releases/download/v${HYPERSDK_VERSION}/tokenvm_${HYPERSDK_VERSION}_linux_amd64.tar.gz"
mkdir tmp-hypersdk
tar -xvf tokenvm_${HYPERSDK_VERSION}_linux_amd64.tar.gz -C tmp-hypersdk
rm -rf tokenvm_${HYPERSDK_VERSION}_linux_amd64.tar.gz
mv tmp-hypersdk/tokenvm /tmp/avalanche-ops/tokenvm
rm -rf tmp-hypersdk

# Setup genesis and configuration files
cat <<EOF > /tmp/avalanche-ops/tokenvm-subnet-config.json
{
  "proposerMinBlockDelay": 0,
  "proposerNumHistoricalBlocks": 768
}
EOF
cat /tmp/avalanche-ops/tokenvm-subnet-config.json

# TODO: make address configurable via ENV
cat <<EOF > /tmp/avalanche-ops/allocations.json
[{"address":"token1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97nsjzf3yp", "balance":1000000000000000}]
EOF

# TODO: make fee params configurable via ENV
/tmp/avalanche-ops/token-cli genesis generate /tmp/avalanche-ops/allocations.json \
--genesis-file /tmp/avalanche-ops/tokenvm-genesis.json \
--max-block-units 18446744073709551615,18446744073709551615,18446744073709551615,18446744073709551615,18446744073709551615 \
--window-target-units 1800000,18446744073709551615,18446744073709551615,18446744073709551615,18446744073709551615 \
--min-block-gap 250
cat /tmp/avalanche-ops/tokenvm-genesis.json

cat <<EOF > /tmp/avalanche-ops/tokenvm-chain-config.json
{
  "mempoolSize": 10000000,
  "mempoolPayerSize": 10000000,
  "mempoolExemptPayers":["token1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97nsjzf3yp"],
  "streamingBacklogSize": 10000000,
  "storeTransactions": false,
  "verifySignatures": true,
  "trackedPairs":["*"],
  "logLevel": "info",
}
EOF
cat /tmp/avalanche-ops/tokenvm-chain-config.json

# Plan network deploy
echo 'what is your <AWS_PROFILE_NAME>?'
read AWS_PROFILE_NAME

echo 'planning DEVNET deploy...'
/tmp/avalancheup-aws default-spec \
--arch-type amd64 \
--os-type ubuntu20.04 \
--anchor-nodes 3 \
--non-anchor-nodes 7 \
--regions us-west-2 \
--instance-mode=on-demand \
--instance-types='{"us-west-2":["c5.4xlarge"]}' \
--ip-mode=ephemeral \
--metrics-fetch-interval-seconds 60 \
--network-name custom \
--avalanchego-release-tag v${AVALANCHEGO_VERSION} \
--create-dev-machine \
--keys-to-generate 5 \
--subnet-config-file /tmp/avalanche-ops/tokenvm-subnet-config.json \
--vm-binary-file /tmp/avalanche-ops/tokenvm \
--chain-name tokenvm \
--chain-genesis-file /tmp/avalanche-ops/tokenvm-genesis.json \
--chain-config-file /tmp/avalanche-ops/tokenvm-chain-config.json \
--spec-file-path /tmp/avalanche-ops/spec.yml \
--profile-name ${AWS_PROFILE_NAME}

# Update YAML Spec File
yq -i '.avalanchego_config.network-compression-type = "zstd"'  /tmp/avalanche-ops/spec.yml

# Deploy DEVNET

# Configure token-cli

# Sign into dev machine, download token-cli, start Prometheus

# Print command for running spam script inside of this machine
