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
CLI_COMMIT=v1.4.3-rc.0
cd $TMPDIR
git clone https://github.com/ava-labs/avalanche-cli
cd avalanche-cli
git checkout $CLI_COMMIT
./scripts/build.sh
mv ./bin/avalanche "${TMPDIR}/avalanche"
cd $pw

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
  "authExecutionCores": 30,
  "actionExecutionCores": 4,
  "rootGenerationCores": 34,
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
function cleanup {
  RED='\033[0;31m'
  NC='\033[0m'
  echo -e "${RED}To destroy the devnet, run: \"${TMPDIR}/avalanche node destroy ${CLUSTER}\"${NC}"
}
trap cleanup EXIT
# TODO: re-add monitoring when properly configure grafana/add load tester to it
$TMPDIR/avalanche node devnet wiz ${CLUSTER} ${VMID} --node-type c5.9xlarge --num-apis 1,1 --num-validators 2,2 --region us-east-1,us-east-2 --aws --use-static-ip=false --skip-monitoring --default-validator-params --custom-vm-repo-url="https://www.github.com/ava-labs/hypersdk" --custom-vm-branch $VM_COMMIT --custom-vm-build-script="examples/morpheusvm/scripts/build.sh" --custom-subnet=true --subnet-genesis="${TMPDIR}/morpheusvm.genesis" --subnet-config="${TMPDIR}/morpheusvm.genesis" --chain-config="${TMPDIR}/morpheusvm.config" --node-config="${TMPDIR}/node.config" --remote-cli-version $CLI_COMMIT

echo "Cluster info: (~/.avalanche-cli/nodes/inventories/${CLUSTER}/clusterInfo.yaml)"
cat ~/.avalanche-cli/nodes/inventories/$CLUSTER/clusterInfo.yaml

# Import the cluster into morpheus-cli for local interaction
echo "Importing cluster into local morpheus-cli"
$TMPDIR/morpheus-cli chain import-cli ~/.avalanche-cli/nodes/inventories/$CLUSTER/clusterInfo.yaml
echo "Run this command in a separate window to monitor cluster: ${TMPDIR}/morpheus-cli prometheus generate"

# Wait for user to confirm that they want to launch load test
echo "Start load test (y/n)?:"

# Wait for the user to press a key
read -s -n 1 key

# Check which key was pressed
case $key in
    y|Y)
        echo "You typed 'y'. Starting..."
        ;;
    n|N)
        echo "You typed 'n'. Exiting..."
        exit 1
        ;;
    *)
        echo "Invalid input. Please type 'y' or 'n'."
        ;;
esac

# Start load test on dedicated machine
/bin/avalanche node loadtest ${CLUSTER} ${VMID} --loadTestRepoURL="https://github.com/ava-labs/hypersdk/commit/${VM_COMMIT}" --loadTestBuildCmd="cd /home/ubuntu/hypersdk/examples/morpheusvm; CGO_CFLAGS=\"-O -D__BLST_PORTABLE__\" go build -o ~/simulator ./cmd/morpheus-cli" --loadTestCmd="./home/ubuntu/simulator spam run ed25519 --max-tx-backlog=600000 --num-accounts=2500 --num-txs=20 --num-clients=5 --cluster-info=/home/ubuntu/clusterInfo.yaml --private-key=323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7" --remote-cli-version $CLI_COMMIT
