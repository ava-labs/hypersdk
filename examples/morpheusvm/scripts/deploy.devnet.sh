#!/usr/bin/env bash
# Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -e

# Ensure we return back to the original directory
pw=$(pwd)
function cleanup() {
  cd "$pw"
}
trap cleanup EXIT

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

# Install avalanche-cli
CLI_COMMIT=v1.4.3-rc.1
cd $TMPDIR
git clone https://github.com/ava-labs/avalanche-cli
cd avalanche-cli
git checkout ef0a7b748fdd5d6d91fa52b2cfe822f08008d4fe # TODO: replace with CLI_COMMIT
./scripts/build.sh
mv ./bin/avalanche "${TMPDIR}/avalanche"
cd $pw

# Build morpheus-cli
echo "building morpheus-cli"
go build -v -o "${TMPDIR}"/morpheus-cli ./cmd/morpheus-cli

# Generate genesis file and configs
#
# We use a shorter EPOCH_DURATION and VALIDITY_WINDOW to speed up devnet
# startup. In a production environment, these should be set to longer values.
#
# TODO: print this out while waiting for network as well.
ADDRESS=morpheus1qrzvk4zlwj9zsacqgtufx7zvapd3quufqpxk5rsdd4633m4wz2fdjk97rwu
EPOCH_DURATION=30000
VALIDITY_WINDOW=25000
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
--epoch-duration "${EPOCH_DURATION}" \
--validity-window "${VALIDITY_WINDOW}" \
--min-unit-price "${MIN_UNIT_PRICE}" \
--max-chunk-units "${MAX_CHUNK_UNITS}" \
--min-block-gap "${MIN_BLOCK_GAP}" \
--genesis-file "${TMPDIR}"/morpheusvm.genesis

# TODO: find a smarter way to split auth cores between exec and RPC
cat <<EOF > "${TMPDIR}"/morpheusvm.config
{
  "chunkBuildFrequency": 750,
  "targetChunkBuildDuration": 500,
  "blockBuildFrequency": 100,
  "mempoolSize": 10000000,
  "mempoolSponsorSize": 10000000,
  "mempoolExemptSponsors":["${ADDRESS}"],
  "authExecutionCores": 30,
  "actionExecutionCores": 8,
  "rootGenerationCores": 32,
  "missingChunkFetchers": 48,
  "verifyAuth":true,
  "authRPCCores": 24,
  "authRPCBacklog": 10000000,
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
  "log-level":"INFO",
  "log-display-level":"INFO",
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
  "consensus-app-concurrency":"128",
  "profile-continuous-enabled":true,
  "profile-continuous-freq":"1m",
  "http-host":"",
  "http-allowed-origins": "*",
  "http-allowed-hosts": "*"
}
EOF

# Setup devnet
CLUSTER="vryx-$(date +%s)"
VMID=$(git rev-parse --short HEAD) # ensure we use a fresh vm
VM_COMMIT=$(git rev-parse HEAD)
RED='\033[0;31m'
YELLOw='\033[1;33m'
NC='\033[0m'
function cleanup {
  echo -e "${RED}To destroy the devnet, run:${NC} \"${TMPDIR}/avalanche node destroy ${CLUSTER}\""
}
trap cleanup EXIT
$TMPDIR/avalanche node devnet wiz ${CLUSTER} ${VMID} --aws --node-type c7g.8xlarge --num-apis 1,1,1,1,1 --num-validators 2,2,2,2,2 --region us-east-1,eu-west-1,us-west-1,ap-northeast-2,ca-central-1 --use-static-ip=false --enable-monitoring=true --default-validator-params --custom-vm-repo-url="https://www.github.com/ava-labs/hypersdk" --custom-vm-branch $VM_COMMIT --custom-vm-build-script="examples/morpheusvm/scripts/build.sh" --custom-subnet=true --subnet-genesis="${TMPDIR}/morpheusvm.genesis" --subnet-config="${TMPDIR}/morpheusvm.genesis" --chain-config="${TMPDIR}/morpheusvm.config" --node-config="${TMPDIR}/node.config" --remote-cli-version $CLI_COMMIT

echo "Cluster info: (~/.avalanche-cli/nodes/inventories/${CLUSTER}/clusterInfo.yaml)"
cat ~/.avalanche-cli/nodes/inventories/$CLUSTER/clusterInfo.yaml

# Import the cluster into morpheus-cli for local interaction
echo "Importing cluster into local morpheus-cli"
$TMPDIR/morpheus-cli chain import-cli ~/.avalanche-cli/nodes/inventories/$CLUSTER/clusterInfo.yaml
echo -e "\n${YELLOW}Run this command in a separate window to monitor cluster:${NC} \"${TMPDIR}/morpheus-cli prometheus generate\""

# Wait for user to confirm that they want to launch load test
while true
do
  echo -n "Start load test (y/n)?: "

  # Wait for the user to press a key
  read -s -n 1 key

  # Check which key was pressed
  case $key in
      y|Y)
          printf "y\n"
          break
          ;;
      n|N)
          printf "n\nExiting...\n"
          exit 1
          ;;
      *)
          printf "\nInvalid input. Please type 'y' or 'n'.\n"
          ;;
  esac
done

# Wait for epoch initialization
SLEEP_DUR=$(($EPOCH_DURATION / 1000 * 2))
echo "Waiting for epoch initialization ($SLEEP_DUR seconds)..."
echo -e "${YELLOW}We use a shorter EPOCH_DURATION to speed up devnet startup. In a production environment, this should be set to a longer value.${NC}"
sleep $SLEEP_DUR

# Start load test on dedicated machine
# TODO: only start using again once test is run async and logs are collected using Loki (stream isn't reliable)
$TMPDIR/avalanche node loadtest ${CLUSTER} ${VMID} --loadTestRepoURL="https://github.com/ava-labs/hypersdk/commit/${VM_COMMIT}" --loadTestBuildCmd="cd /home/ubuntu/hypersdk/examples/morpheusvm; CGO_CFLAGS=\"-O -D__BLST_PORTABLE__\" go build -o ~/simulator ./cmd/morpheus-cli" --loadTestCmd="exit"

echo -e "${YELLOW}To run load test, ssh into monitoring instance run this command:${NC} /home/ubuntu/simulator spam run ed25519 --num-accounts=10000 --txs-per-second=100000 --num-clients=5 --cluster-info=/home/ubuntu/clusterInfo.yaml --private-key=323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7"
