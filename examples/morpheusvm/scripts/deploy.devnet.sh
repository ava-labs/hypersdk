#!/usr/bin/env bash
# Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -e

# Set the CGO flags to use the portable version of BLST
#
# We use "export" here instead of just setting a bash variable because we need
# to pass this flag to all child processes spawned by the shell.
export CGO_CFLAGS="-O -D__BLST_PORTABLE__"

# Set console colors
RED='\033[0;31m'
YELLOW='\033[1;33m'
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
TMPDIR=/tmp/morpheusvm-deploy
rm -rf $TMPDIR && mkdir -p $TMPDIR
echo -e "${YELLOW}set working directory:${NC} $TMPDIR"

# Install avalanche-cli
LOCAL_CLI_COMMIT=ea581bbf6a0e622ef985e0e66cf15b493a707b05
REMOTE_CLI_COMMIT=v1.4.3-rc.2
cd $TMPDIR
git clone https://github.com/ava-labs/avalanche-cli
cd avalanche-cli
git checkout $LOCAL_CLI_COMMIT
./scripts/build.sh
mv ./bin/avalanche "${TMPDIR}/avalanche"
cd $pw

# Install morpheus-cli
MORPHEUS_VM_COMMIT=66b6c817628bbe45dc94f748c576fe5349dbe57a
echo -e "${YELLOW}building morpheus-cli${NC}"
cd $TMPDIR
git clone https://github.com/ava-labs/hypersdk
cd hypersdk
git checkout $MORPHEUS_VM_COMMIT
VMID=$(git rev-parse --short HEAD) # ensure we use a fresh vm
VM_COMMIT=$(git rev-parse HEAD)
cd examples/morpheusvm
./scripts/build.sh
mv ./build/morpheus-cli "${TMPDIR}/morpheus-cli"
cd $pw

# Generate genesis file and configs
#
# We use a shorter EPOCH_DURATION and VALIDITY_WINDOW to speed up devnet
# startup. In a production environment, these should be set to longer values.
#
# Addresses:
# morpheus1qrzvk4zlwj9zsacqgtufx7zvapd3quufqpxk5rsdd4633m4wz2fdjk97rwu: 323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7
# morpheus1qryyvfut6td0l2vwn8jwae0pmmev7eqxs2vw0fxpd2c4lr37jj7wvrj4vc3: ee11a050c75f0f47390f8ed98ab29fbce8c1f820b0245af56e1cb484a80c8022d77899baf0059747b8b685cfe62296f85f67083dc0bf8d2fab24c5ee3a7563b9
# morpheus1qp52zjc3ul85309xn9stldfpwkseuth5ytdluyl7c5mvsv7a4fc76g6c4w4: 34214e27f4c7d17315694968e37d999b848bb7b0bc95d679eb8163cf516c15dd9e77d9ebe639f9bece4260f4cce91ccf365dbce726da4299ff5a1b1ed31b339e
# morpheus1qzqjp943t0tudpw06jnvakdc0y8w790tzk7suc92aehjw0epvj93s0uzasn: ba09c65939a182f46879fcda172eabe9844d1f0a835a00c905dd2fa11b61a50ff38c9fdaef41e74730a732208284f2199fcd2f31779942662139884ca3f97a77
# morpheus1qz97wx3vl3upjuquvkulp56nk20l3jumm3y4yva7v6nlz5rf8ukty8fh27r: 3e5ab8a792187c8fa0a87e2171058d9a0c16ca07bc35c2cfb5e2132078fe18c0a70d00475d1e86ef32bb22397e47722c420dd4caf157400b83d9262af6bf0af5
EPOCH_DURATION=60000
VALIDITY_WINDOW=59000
MIN_BLOCK_GAP=1000
MIN_UNIT_PRICE="1,1,1,1,1"
MAX_UINT64=18446744073709551615
MAX_CHUNK_UNITS="1800000,${MAX_UINT64},${MAX_UINT64},${MAX_UINT64},${MAX_UINT64}" # in a load test, all we care about is that chunks are size-bounded (2MB network limit)
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
  "chunkBuildFrequency": 333,
  "targetChunkBuildDuration": 250,
  "blockBuildFrequency": 100,
  "mempoolSize": 2147483648,
  "mempoolSponsorSize": 10000000,
  "authExecutionCores": 32,
  "precheckCores": 32,
  "actionExecutionCores": 32,
  "rootGenerationCores": 32,
  "missingChunkFetchers": 48,
  "verifyAuth":true,
  "authRPCCores": 48,
  "authRPCBacklog": 10000000,
  "authGossipCores": 32,
  "authGossipBacklog": 10000000,
  "streamingBacklogSize": 10000000,
  "continuousProfilerDir":"/home/ubuntu/morpheusvm-profiles",
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
  "throttler-inbound-node-max-processing-msgs":"1000000",
	"throttler-inbound-node-max-at-large-bytes":"10737418240",
  "throttler-inbound-bandwidth-refill-rate":"1073741824",
  "throttler-inbound-bandwidth-max-burst-size":"1073741824",
  "throttler-inbound-cpu-validator-alloc":"100000",
  "throttler-inbound-disk-validator-alloc":"10737418240000",
  "throttler-outbound-validator-alloc-size":"10737418240",
  "throttler-outbound-at-large-alloc-size":"10737418240",
  "throttler-outbound-node-max-at-large-bytes":"10737418240",
  "consensus-on-accept-gossip-validator-size":"10",
  "consensus-on-accept-gossip-peer-size":"10",
  "network-compression-type":"zstd",
  "consensus-app-concurrency":"16384",
  "profile-continuous-enabled":true,
  "profile-continuous-freq":"1m",
  "http-host":"",
  "http-allowed-origins": "*",
  "http-allowed-hosts": "*"
}
EOF

# Setup devnet
CLUSTER="vryx-$(date +%s)"
function cleanup {
  echo -e "${RED}run this command to destroy the devnet:${NC} ${TMPDIR}/avalanche node destroy ${CLUSTER}"
}
trap cleanup EXIT
# List of supported instances in each AWS region: https://docs.aws.amazon.com/ec2/latest/instancetypes/ec2-instance-regions.html
#
# It is not recommended to use an instance with burstable network performance.
echo -e "${YELLOW}creating devnet${NC}"
$TMPDIR/avalanche node devnet wiz ${CLUSTER} ${VMID} --aws --node-type m7g.8xlarge --num-apis 1,1,1,1,1 --num-validators 5,5,5,5,5 --region us-west-2,us-east-1,ap-south-1,ap-northeast-1,eu-west-1 --use-static-ip=false --enable-monitoring --default-validator-params --custom-vm-repo-url="https://www.github.com/ava-labs/hypersdk" --custom-vm-branch $VM_COMMIT --custom-vm-build-script="examples/morpheusvm/scripts/build.sh" --custom-subnet=true --subnet-genesis="${TMPDIR}/morpheusvm.genesis" --subnet-config="${TMPDIR}/morpheusvm.genesis" --chain-config="${TMPDIR}/morpheusvm.config" --node-config="${TMPDIR}/node.config" --remote-cli-version $REMOTE_CLI_COMMIT --add-grafana-dashboard="${TMPDIR}/hypersdk/examples/morpheusvm/grafana.json"
EPOCH_WAIT_START=$(date +%s)

# Import the cluster into morpheus-cli for local interaction
$TMPDIR/morpheus-cli chain import-cli ~/.avalanche-cli/nodes/inventories/$CLUSTER/clusterInfo.yaml

# Point to cluster dashboard
echo -e "${YELLOW}devnet dashboard:${NC} http://$(yq e '.MONITOR.IP' ~/.avalanche-cli/nodes/inventories/$CLUSTER/clusterInfo.yaml):3000/d/vryx-poc (username: admin, password: admin)"

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
EPOCH_WAIT_END=$(date +%s)
TIME_TAKEN=$((EPOCH_WAIT_END - EPOCH_WAIT_START))

# Wait for epoch initialization
SLEEP_DUR=$(($EPOCH_DURATION / 1000 * 3))
if [ $TIME_TAKEN -lt $SLEEP_DUR ]; then
  SLEEP_DUR=$(($SLEEP_DUR - $TIME_TAKEN))
  echo "Waiting for epoch initialization ($SLEEP_DUR seconds)..."
  sleep $SLEEP_DUR
fi

# Start load test on dedicated machine
#
# Zipf parameters expected to lead to ~1M active accounts per 60s
echo -e "${YELLOW}starting load test${NC}"
$TMPDIR/avalanche node loadtest start "default" ${CLUSTER} ${VMID} --region eu-west-1 --aws --node-type c7gn.8xlarge --load-test-repo="https://github.com/ava-labs/hypersdk" --load-test-branch=$VM_COMMIT --load-test-build-cmd="cd /home/ubuntu/hypersdk/examples/morpheusvm; CGO_CFLAGS=\"-O -D__BLST_PORTABLE__\" go build -o ~/simulator ./cmd/morpheus-cli" --load-test-cmd="/home/ubuntu/simulator spam run ed25519 --accounts=10000000 --txs-per-second=100000 --min-capacity=10000 --step-size=1000 --s-zipf=1.2 --v-zipf=2.7 --conns-per-host=10 --cluster-info=/home/ubuntu/clusterInfo.yaml --private-key=323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7"
echo -e "${YELLOW}load test logs:${NC} http://$(yq e '.MONITOR.IP' ~/.avalanche-cli/nodes/inventories/$CLUSTER/clusterInfo.yaml):3000/d/avalanche-loki-logs?var-app=loadtest (username: admin, password: admin)"
echo -e "${YELLOW}run this command to stop load test:${NC} ${TMPDIR}/avalanche node loadtest stop ${CLUSTER} --load-test=\"default\""
