#!/usr/bin/env bash
# shellcheck disable=SC2086,SC2248,SC2317
# Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -e

export CGO_CFLAGS="-O -D__BLST_PORTABLE__"

# project name is expected to be the name of the directory containing the project
# it should be located in the examples directory of the hypersdk.
PROJECT_NAME="morpheusvm"

# Name of the cli binary for the project
PROJECT_CLI_NAME="morpheus-cli"

# Commit of the MorpheusVM repository to build and install
PROJECT_VM_COMMIT=fd36681b939cd28b26526eaf1e5089c2c21fb91d

# Set AvalancheGo Build (should have canPop disabled)
AVALANCHEGO_COMMIT=94be34ac64cba099d7be198965f0dd551f7cdefb

# Install avalanche-cli
LOCAL_CLI_COMMIT=18e5259b499bb9b487bb5e1e620d3107e68f2171

# Set console colors
RED='\033[1;31m'
YELLOW='\033[1;33m'
CYAN='\033[1;36m'
NC='\033[0m'

# Ensure we return back to the original directory
pw=$(pwd)
function cleanup() {
  cd "$pw"
}
trap cleanup EXIT

# Ensure that the script is being run from the repository root
if ! [[ "$0" =~ scripts/deploy.devnet.sh ]]; then
  echo -e "${RED}must be run from repository root${NC}"
  exit 1
fi

# Ensure required software is installed and aws credentials are set
if ! command -v go >/dev/null 2>&1 ; then
    echo -e "${RED}golang is not installed. exiting...${NC}"
    exit 1
fi

if ! aws sts get-caller-identity >/dev/null 2>&1 ; then
    echo -e "${RED}aws credentials not set. exiting...${NC}"
    exit 1
fi

# Create temporary directory for the deployment
TMPDIR=/tmp/$PROJECT_NAME-deploy
rm -rf $TMPDIR && mkdir -p $TMPDIR
echo -e "${YELLOW}set working directory:${NC} $TMPDIR"

cd $TMPDIR
git clone https://github.com/ava-labs/avalanche-cli
cd avalanche-cli
git checkout $LOCAL_CLI_COMMIT
./scripts/build.sh
mv ./bin/avalanche "${TMPDIR}/avalanche"
cd $pw

# Install and build the project and project CLI
echo -e "${YELLOW}building ${PROJECT_CLI_NAME}${NC}"
cd $TMPDIR
git clone https://github.com/furkan-ux/hypersdk
cd hypersdk
git checkout $PROJECT_VM_COMMIT
VMID=$(git rev-parse --short HEAD) # ensure we use a fresh vm
VM_COMMIT=$(git rev-parse HEAD)
cd examples/$PROJECT_NAME
./scripts/build.sh
# build project cli
go build -o build/$PROJECT_CLI_NAME cmd/$PROJECT_CLI_NAME/*.go
mv ./build/$PROJECT_CLI_NAME "${TMPDIR}/${PROJECT_CLI_NAME}"
cd $pw

# Generate genesis file and configs
#
# We use a shorter VALIDITY_WINDOW to speed up devnet
# startup. In a production environment, these should be set to longer values.
#
# Addresses:
# 323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7
# ee11a050c75f0f47390f8ed98ab29fbce8c1f820b0245af56e1cb484a80c8022d77899baf0059747b8b685cfe62296f85f67083dc0bf8d2fab24c5ee3a7563b9
# 34214e27f4c7d17315694968e37d999b848bb7b0bc95d679eb8163cf516c15dd9e77d9ebe639f9bece4260f4cce91ccf365dbce726da4299ff5a1b1ed31b339e
# ba09c65939a182f46879fcda172eabe9844d1f0a835a00c905dd2fa11b61a50ff38c9fdaef41e74730a732208284f2199fcd2f31779942662139884ca3f97a77
# 3e5ab8a792187c8fa0a87e2171058d9a0c16ca07bc35c2cfb5e2132078fe18c0a70d00475d1e86ef32bb22397e47722c420dd4caf157400b83d9262af6bf0af5
VALIDITY_WINDOW=59000
MIN_BLOCK_GAP=1000
MIN_UNIT_PRICE="1,1,1,1,1"
MAX_UINT64=18446744073709551615
MAX_CHUNK_UNITS="1800000,${MAX_UINT64},${MAX_UINT64},${MAX_UINT64},${MAX_UINT64}" # in a load test, all we care about is that chunks are size-bounded (2MB network limit)
# Sum of allocations must be less than uint64 max
cat <<EOF > "${TMPDIR}"/allocations.json
[
  {"address":"0x00c4cb545f748a28770042f893784ce85b107389004d6a0e0d6d7518eeae1292d914969017", "balance":3000000000000000000},
  {"address":"0x003020338128fc7babb4e5850aace96e589f55b33bda90d62c44651de110ea5b8c0b5ee37f", "balance":3000000000000000000},
  {"address":"0x006cf906f2df7c34d9be247dd384aefb43510d37d6c9ab273199d68c3b85130bcd05081a2c", "balance":3000000000000000000},
  {"address":"0x00f45cd0197c148ebd317efe15ef91df1af7b5092f01f6e36cd4063c2e6ee149403f724a12", "balance":3000000000000000000},
  {"address":"0x00e3ae18f245e3bbb50860db2f44a0fe460e1de92b698d6370ec3112501ec6d9ba26a9d456", "balance":3000000000000000000}
]
EOF

"${TMPDIR}/${PROJECT_CLI_NAME}" genesis generate "${TMPDIR}"/allocations.json \
--validity-window "${VALIDITY_WINDOW}" \
--min-unit-price "${MIN_UNIT_PRICE}" \
--max-block-units "${MAX_CHUNK_UNITS}" \
--min-block-gap "${MIN_BLOCK_GAP}" \
--genesis-file "${TMPDIR}/${PROJECT_NAME}".genesis

# TODO: find a smarter way to split auth cores between exec and RPC
# TODO: we limit root generation cores because it can cause network handling to stop (exhausts all CPU for a few seconds)
cat <<EOF > "${TMPDIR}/${PROJECT_NAME}".config
{
  "chunkBuildFrequency": 250,
  "targetChunkBuildDuration": 250,
  "blockBuildFrequency": 100,
  "mempoolSize": 2147483648,
  "mempoolSponsorSize": 10000000,
  "authExecutionCores": 16,
  "precheckCores": 16,
  "actionExecutionCores": 8,
  "missingChunkFetchers": 48,
  "verifyAuth": true,
  "authRPCCores": 48,
  "authRPCBacklog": 10000000,
  "authGossipCores": 16,
  "authGossipBacklog": 10000000,
  "chunkStorageCores": 16,
  "chunkStorageBacklog": 10000000,
  "streamingBacklogSize": 10000000,
  "continuousProfilerDir":"/home/ubuntu/${PROJECT_NAME}-profiles",
  "logLevel": "INFO"
}
EOF

cat <<EOF > "${TMPDIR}/${PROJECT_NAME}".subnet
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
  "throttler-inbound-node-max-processing-msgs":"1000000",
  "throttler-inbound-node-max-at-large-bytes":"10737418240",
  "throttler-inbound-bandwidth-refill-rate":"1073741824",
  "throttler-inbound-bandwidth-max-burst-size":"1073741824",
  "throttler-inbound-cpu-validator-alloc":"100000",
  "throttler-inbound-cpu-max-non-validator-usage":"100000",
  "throttler-inbound-cpu-max-non-validator-node-usage":"100000",
  "throttler-inbound-disk-validator-alloc":"10737418240000",
  "throttler-outbound-validator-alloc-size":"10737418240",
  "throttler-outbound-at-large-alloc-size":"10737418240",
  "throttler-outbound-node-max-at-large-bytes":"10737418240",
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
CLUSTER="${PROJECT_NAME}-$(date +%s)"

interrupted=false
function showcleanup {
   if [ "$interrupted" = false ]; then
        echo -e "\n\n${RED}run this command to destroy the devnet:${NC} ${TMPDIR}/avalanche node destroy ${CLUSTER}\n"
   fi
}

function cleanup {
  interrupted=true
  echo -e "\n\n${RED}destroying the devnet, running:${NC} ${TMPDIR}/avalanche node destroy ${CLUSTER}\n"
  ${TMPDIR}/avalanche node destroy ${CLUSTER} -y
}

trap showcleanup EXIT
trap cleanup SIGINT

# List of supported instances in each AWS region: https://docs.aws.amazon.com/ec2/latest/instancetypes/ec2-instance-regions.html
#
# It is not recommended to use an instance with burstable network performance.
echo -e "${YELLOW}creating devnet${NC}"
$TMPDIR/avalanche node devnet wiz ${CLUSTER} ${VMID} --aws --node-type c7g.8xlarge --aws-volume-type=io2 --aws-volume-iops=2500 --aws-volume-size=100 --num-apis 1,1,1,1,1 --num-validators 1,1,1,1,1 --region us-west-1,us-east-1,us-east-2,ap-northeast-1,us-west-2 --use-static-ip=false --auto-replace-keypair --enable-monitoring --default-validator-params --custom-avalanchego-version $AVALANCHEGO_COMMIT --custom-vm-repo-url="https://www.github.com/furkan-ux/hypersdk" --custom-vm-branch $VM_COMMIT --custom-vm-build-script="examples/${PROJECT_NAME}/scripts/build.sh" --custom-subnet=true --subnet-genesis="${TMPDIR}/${PROJECT_NAME}.genesis" --subnet-config="${TMPDIR}/${PROJECT_NAME}.genesis" --chain-config="${TMPDIR}/${PROJECT_NAME}.config" --node-config="${TMPDIR}/node.config" --skip-update-check --add-grafana-dashboard="${TMPDIR}/hypersdk/examples/${PROJECT_NAME}/grafana.json"

# Import the cluster into project cli for local interaction
$TMPDIR/$PROJECT_CLI_NAME chain import-cli ~/.avalanche-cli/nodes/inventories/$CLUSTER/clusterInfo.yaml

# Start load test on dedicated machine
#
# Zipf parameters expected to lead to ~1M active accounts per 60s
echo -e "\n${YELLOW}starting load test...${NC}"
$TMPDIR/avalanche node loadtest start "default" ${CLUSTER} ${VMID} --region us-west-2 --aws --node-type c7gn.8xlarge --load-test-repo="https://github.com/furkan-ux/hypersdk" --load-test-branch=$VM_COMMIT --load-test-build-cmd="cd /home/ubuntu/hypersdk/examples/${PROJECT_NAME}; CGO_CFLAGS=\"-O -D__BLST_PORTABLE__\" go build -o ~/simulator ./cmd/${PROJECT_CLI_NAME}" --load-test-cmd="/home/ubuntu/simulator spam run ed25519 --cluster-info=/home/ubuntu/clusterInfo.yaml --key=323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7"

# Log dashboard information
echo -e "\n\n${CYAN}dashboards:${NC} (username: admin, password: admin)"
echo "* hypersdk (metrics): http://$(yq e '.MONITOR.IP' ~/.avalanche-cli/nodes/inventories/$CLUSTER/clusterInfo.yaml):3000/d/load-test-dashboard"
echo "* hypersdk (logs): http://$(yq e '.MONITOR.IP' ~/.avalanche-cli/nodes/inventories/$CLUSTER/clusterInfo.yaml):3000/d/avalanche-loki-logs/avalanche-logs?var-app=subnet"
echo "* load test (logs): http://$(yq e '.MONITOR.IP' ~/.avalanche-cli/nodes/inventories/$CLUSTER/clusterInfo.yaml):3000/d/avalanche-loki-logs/avalanche-logs?var-app=loadtest"