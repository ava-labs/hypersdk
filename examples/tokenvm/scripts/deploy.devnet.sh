#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

# Ensure we return back to current directory
pw=$(pwd)
function cleanup() {
  cd $pw
}
trap cleanup EXIT

# Setup cache
BUST_CACHE=${BUST_CACHE:-false}
if ${BUST_CACHE}; then
  rm -rf /tmp/avalanche-ops-cache
fi
mkdir -p /tmp/avalanche-ops-cache

# Create deployment directory (avalanche-ops creates metadata in cwd)
DATE=$(date '+%m%d%Y-%H%M%S')
DEPLOY_PREFIX=~/avalanche-ops-deploys/${DATE}
mkdir -p ${DEPLOY_PREFIX}
DEPLOY_ARTIFACT_PREFIX=${DEPLOY_PREFIX}/artifacts
mkdir -p ${DEPLOY_ARTIFACT_PREFIX}
echo create deployment folder: ${DEPLOY_PREFIX}
cd ${DEPLOY_PREFIX}

# Set constants
export DEPLOYER_ARCH_TYPE=$(uname -m)
[ $DEPLOYER_ARCH_TYPE = x86_64 ] && DEPLOYER_ARCH_TYPE=amd64
echo DEPLOYER_ARCH_TYPE: ${DEPLOYER_ARCH_TYPE}
export DEPLOYER_OS_TYPE=$(uname | tr '[:upper:]' '[:lower:]')
echo DEPLOYER_OS_TYPE: ${DEPLOYER_OS_TYPE}
export AVALANCHEGO_VERSION="1.10.12"
echo AVALANCHEGO_VERSION: ${AVALANCHEGO_VERSION}
export HYPERSDK_VERSION="0.0.15-rc.1"
echo HYPERSDK_VERSION: ${HYPERSDK_VERSION}
# TODO: set deploy os/arch

# Check valid setup
if [ ${DEPLOYER_OS_TYPE} != 'darwin' ]; then
  echo 'os is not supported' >&2
  exit 1
fi
if [ ${DEPLOYER_ARCH_TYPE} != 'arm64' ]; then
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
if [ -f /tmp/avalanche-ops-cache/avalancheup-aws ]; then
  cp /tmp/avalanche-ops-cache/avalancheup-aws ${DEPLOY_ARTIFACT_PREFIX}/avalancheup-aws
  echo 'found avalanche-ops in cache'
else
  wget https://github.com/ava-labs/avalanche-ops/releases/download/latest/avalancheup-aws.aarch64-apple-darwin
  mv ./avalancheup-aws.aarch64-apple-darwin ${DEPLOY_ARTIFACT_PREFIX}/avalancheup-aws
  chmod +x ${DEPLOY_ARTIFACT_PREFIX}/avalancheup-aws
  cp ${DEPLOY_ARTIFACT_PREFIX}/avalancheup-aws /tmp/avalanche-ops-cache/avalancheup-aws
fi
${DEPLOY_ARTIFACT_PREFIX}/avalancheup-aws --help

# Install token-cli
echo 'installing token-cli...'
if [ -f /tmp/avalanche-ops-cache/token-cli ]; then
  cp /tmp/avalanche-ops-cache/token-cli ${DEPLOY_ARTIFACT_PREFIX}/token-cli
  echo 'found token-cli in cache'
else
  wget "https://github.com/ava-labs/hypersdk/releases/download/v${HYPERSDK_VERSION}/tokenvm_${HYPERSDK_VERSION}_${DEPLOYER_OS_TYPE}_${DEPLOYER_ARCH_TYPE}.tar.gz"
  mkdir -p /tmp/token-installs
  tar -xvf tokenvm_${HYPERSDK_VERSION}_${DEPLOYER_OS_TYPE}_${DEPLOYER_ARCH_TYPE}.tar.gz -C /tmp/token-installs
  rm -rf tokenvm_${HYPERSDK_VERSION}_${DEPLOYER_OS_TYPE}_${DEPLOYER_ARCH_TYPE}.tar.gz
  mv /tmp/token-installs/token-cli ${DEPLOY_ARTIFACT_PREFIX}/token-cli
  rm -rf /tmp/token-installs
  cp ${DEPLOY_ARTIFACT_PREFIX}/token-cli /tmp/avalanche-ops-cache/token-cli
fi

# Download tokenvm
echo 'downloading tokenvm...'
if [ -f /tmp/avalanche-ops-cache/tokenvm ]; then
  cp /tmp/avalanche-ops-cache/tokenvm ${DEPLOY_ARTIFACT_PREFIX}/tokenvm
  cp /tmp/avalanche-ops-cache/token-cli-dev ${DEPLOY_ARTIFACT_PREFIX}/token-cli-dev
  echo 'found tokenvm in cache'
else
  wget "https://github.com/ava-labs/hypersdk/releases/download/v${HYPERSDK_VERSION}/tokenvm_${HYPERSDK_VERSION}_linux_amd64.tar.gz"
  mkdir -p /tmp/token-installs
  tar -xvf tokenvm_${HYPERSDK_VERSION}_linux_amd64.tar.gz -C /tmp/token-installs
  rm -rf tokenvm_${HYPERSDK_VERSION}_linux_amd64.tar.gz
  mv /tmp/token-installs/tokenvm ${DEPLOY_ARTIFACT_PREFIX}/tokenvm
  mv /tmp/token-installs/token-cli ${DEPLOY_ARTIFACT_PREFIX}/token-cli-dev
  rm -rf /tmp/token-installs
  cp ${DEPLOY_ARTIFACT_PREFIX}/tokenvm /tmp/avalanche-ops-cache/tokenvm
  cp ${DEPLOY_ARTIFACT_PREFIX}/token-cli-dev /tmp/avalanche-ops-cache/token-cli-dev
fi

# Setup genesis and configuration files
cat <<EOF > ${DEPLOY_ARTIFACT_PREFIX}/tokenvm-subnet-config.json
{
  "proposerMinBlockDelay": 0,
  "proposerNumHistoricalBlocks": 50000
}
EOF
cat ${DEPLOY_ARTIFACT_PREFIX}/tokenvm-subnet-config.json

# TODO: make address configurable via ENV
cat <<EOF > ${DEPLOY_ARTIFACT_PREFIX}/allocations.json
[
  {"address":"token1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97nsjzf3yp", "balance":1000000000000000}
]
EOF

# Block bandwidth per second is a function of ~1.8MB * 1/blockGap
#
# TODO: make fee params configurable via ENV
MAX_UINT64=18446744073709551615
${DEPLOY_ARTIFACT_PREFIX}/token-cli genesis generate ${DEPLOY_ARTIFACT_PREFIX}/allocations.json \
--genesis-file ${DEPLOY_ARTIFACT_PREFIX}/tokenvm-genesis.json \
--max-block-units 1800000,${MAX_UINT64},${MAX_UINT64},${MAX_UINT64},${MAX_UINT64} \
--window-target-units ${MAX_UINT64},${MAX_UINT64},${MAX_UINT64},${MAX_UINT64},${MAX_UINT64} \
--min-block-gap 250
cat ${DEPLOY_ARTIFACT_PREFIX}/tokenvm-genesis.json

cat <<EOF > ${DEPLOY_ARTIFACT_PREFIX}/tokenvm-chain-config.json
{
  "logLevel": "info",
  "mempoolSize": 10000000,
  "mempoolPayerSize": 10000000,
  "mempoolExemptPayers":["token1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97nsjzf3yp"],
  "streamingBacklogSize": 10000000,
  "signatureVerificationCores": 4,
  "rootGenerationCores": 4,
  "transactionExecutionCores": 4,
  "storeTransactions": false,
  "verifySignatures": true,
  "trackedPairs":["*"],
  "continuousProfilerDir":"/data/tokenvm-profiles"
}
EOF
cat ${DEPLOY_ARTIFACT_PREFIX}/tokenvm-chain-config.json

# Plan network deploy
if [ ! -f /tmp/avalanche-ops-cache/aws-profile ]; then
  echo 'what is your AWS profile name?'
  read prof_name
  echo ${prof_name} > /tmp/avalanche-ops-cache/aws-profile
fi
AWS_PROFILE_NAME=$(cat "/tmp/avalanche-ops-cache/aws-profile")

# Create spec file
SPEC_FILE=./aops-${DATE}.yml
echo created avalanche-ops spec file: ${SPEC_FILE}

# Create key file dir
KEY_FILES_DIR=keys
mkdir -p ${KEY_FILES_DIR}

# Create dummy metrics file (can't not upload)
# TODO: fix this
cat <<EOF > "${DEPLOY_ARTIFACT_PREFIX}/metrics.yml"
filters:
  - regex: ^*$
EOF
cat ${DEPLOY_ARTIFACT_PREFIX}/metrics.yml

echo 'planning DEVNET deploy...'
# TODO: increase size once dev machine is working
${DEPLOY_ARTIFACT_PREFIX}/avalancheup-aws default-spec \
--arch-type amd64 \
--os-type ubuntu20.04 \
--anchor-nodes 3 \
--non-anchor-nodes 7 \
--regions us-west-2,us-east-2,eu-west-1 \
--instance-mode=on-demand \
--instance-types='{"us-west-2":["c5.4xlarge"],"us-east-2":["c5.4xlarge"],"eu-west-1":["c5.4xlarge"]}' \
--ip-mode=ephemeral \
--metrics-fetch-interval-seconds 0 \
--upload-artifacts-prometheus-metrics-rules-file-path ${DEPLOY_ARTIFACT_PREFIX}/metrics.yml \
--network-name custom \
--staking-amount-in-avax 2000 \
--avalanchego-release-tag v${AVALANCHEGO_VERSION} \
--create-dev-machine \
--keys-to-generate 5 \
--subnet-config-file ${DEPLOY_ARTIFACT_PREFIX}/tokenvm-subnet-config.json \
--vm-binary-file ${DEPLOY_ARTIFACT_PREFIX}/tokenvm \
--chain-name tokenvm \
--chain-genesis-file ${DEPLOY_ARTIFACT_PREFIX}/tokenvm-genesis.json \
--chain-config-file ${DEPLOY_ARTIFACT_PREFIX}/tokenvm-chain-config.json \
--enable-ssh \
--spec-file-path ${SPEC_FILE} \
--key-files-dir ${KEY_FILES_DIR} \
--profile-name ${AWS_PROFILE_NAME}

# Disable rate limits in config
echo 'updating YAML with new rate limits...'
yq -i '.avalanchego_config.throttler-inbound-validator-alloc-size = 10737418240' ${SPEC_FILE}
yq -i '.avalanchego_config.throttler-inbound-at-large-alloc-size = 10737418240' ${SPEC_FILE}
yq -i '.avalanchego_config.throttler-inbound-node-max-processing-msgs = 100000' ${SPEC_FILE}
yq -i '.avalanchego_config.throttler-inbound-bandwidth-refill-rate = 1073741824' ${SPEC_FILE}
yq -i '.avalanchego_config.throttler-inbound-bandwidth-max-burst-size = 1073741824' ${SPEC_FILE}
yq -i '.avalanchego_config.throttler-inbound-cpu-validator-alloc = 100000' ${SPEC_FILE}
yq -i '.avalanchego_config.throttler-inbound-disk-validator-alloc = 10737418240000' ${SPEC_FILE}
yq -i '.avalanchego_config.throttler-outbound-validator-alloc-size = 10737418240' ${SPEC_FILE}
yq -i '.avalanchego_config.throttler-outbound-at-large-alloc-size = 10737418240' ${SPEC_FILE}
yq -i '.avalanchego_config.consensus-on-accept-gossip-validator-size = 10' ${SPEC_FILE}
yq -i '.avalanchego_config.consensus-on-accept-gossip-non-validator-size = 0' ${SPEC_FILE}
yq -i '.avalanchego_config.consensus-on-accept-gossip-peer-size = 10' ${SPEC_FILE}
yq -i '.avalanchego_config.consensus-accepted-frontier-gossip-peer-size = 10' ${SPEC_FILE}
yq -i '.avalanchego_config.consensus-app-concurrency = 8' ${SPEC_FILE}
yq -i '.avalanchego_config.network-compression-type = "zstd"'  ${SPEC_FILE}

# Deploy DEVNET
echo 'deploying DEVNET...'
${DEPLOY_ARTIFACT_PREFIX}/avalancheup-aws apply \
--spec-file-path ${SPEC_FILE}
echo 'DEVNET deployed'

# Prepare dev machine
#
# TODO: prepare 1 dev machine per region
echo 'setting up dev machine...'
ACCESS_KEY=./aops-${DATE}-ec2-access.us-west-2.key
chmod 400 ${ACCESS_KEY}
DEV_MACHINE_IP=$(yq '.dev_machine_ips[0]' ${SPEC_FILE})
until (scp -o "StrictHostKeyChecking=no" -i ${ACCESS_KEY} ${SPEC_FILE} ubuntu@${DEV_MACHINE_IP}:/home/ubuntu/aops.yml)
do
  # During initial setup, ssh access may fail
  echo 'scp failed...trying again'
  sleep 5
done
cd $pw
scp -o "StrictHostKeyChecking=no" -i ${DEPLOY_PREFIX}/${ACCESS_KEY} ${DEPLOY_ARTIFACT_PREFIX}/token-cli-dev ubuntu@${DEV_MACHINE_IP}:/tmp/token-cli
scp -o "StrictHostKeyChecking=no" -i ${DEPLOY_PREFIX}/${ACCESS_KEY} demo.pk ubuntu@${DEV_MACHINE_IP}:/home/ubuntu/demo.pk
scp -o "StrictHostKeyChecking=no" -i ${DEPLOY_PREFIX}/${ACCESS_KEY} scripts/setup.dev-machine.sh ubuntu@${DEV_MACHINE_IP}:/home/ubuntu/setup.sh
ssh -o "StrictHostKeyChecking=no" -i ${DEPLOY_PREFIX}/${ACCESS_KEY} ubuntu@${DEV_MACHINE_IP} /home/ubuntu/setup.sh
echo 'setup dev machine'

# Generate prometheus link
${DEPLOY_ARTIFACT_PREFIX}/token-cli chain import-ops ${DEPLOY_PREFIX}/${SPEC_FILE}
${DEPLOY_ARTIFACT_PREFIX}/token-cli prometheus generate --prometheus-start=false --prometheus-base-uri=http://${DEV_MACHINE_IP}:9090

# Print final logs
cat << EOF
to login to the dev machine, run the following command:

ssh -o "StrictHostKeyChecking no" -i ${DEPLOY_PREFIX}/${ACCESS_KEY} ubuntu@${DEV_MACHINE_IP}

to view activity (on the dev machine), run the following command:

/tmp/token-cli chain watch --hide-txs

to run a spam script (on the dev machine), run the following command:

/tmp/token-cli spam run

to delete all resources (excluding asg/ssm), run the following command:

/tmp/avalancheup-aws delete \
--delete-cloudwatch-log-group \
--delete-s3-objects \
--delete-ebs-volumes \
--delete-elastic-ips \
--spec-file-path ${DEPLOY_PREFIX}/${SPEC_FILE}

to delete all resources, run the following command:

/tmp/avalancheup-aws delete \
--override-keep-resources-except-asg-ssm \
--delete-cloudwatch-log-group \
--delete-s3-objects \
--delete-ebs-volumes \
--delete-elastic-ips \
--spec-file-path ${DEPLOY_PREFIX}/${SPEC_FILE}
EOF
