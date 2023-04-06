## Deploying Devnets
_In the world of Avalanche, we refer to short-lived, test Subnets as Devnets._

### Step 1: Install `avalanche-ops`
[avalanche-ops](https://github.com/ava-labs/avalanche-ops) provides a command-line
interface to setup nodes and install Custom VMs on both custom networks and on
Fuji using [AWS CloudFormation](https://aws.amazon.com/cloudformation/).

You can view a full list of pre-built binaries on the [latest release
page](https://github.com/ava-labs/avalanche-ops/releases/tag/latest).

#### Option 1: Install `avalanche-ops` on Mac
```bash
rm -f /tmp/avalancheup-aws
wget "https://github.com/ava-labs/avalanche-ops/releases/download/latest/avalancheup-aws.aarch64-apple-darwin"
mv ./avalancheup-aws.aarch64-apple-darwin /tmp/avalancheup-aws
chmod +x /tmp/avalancheup-aws
/tmp/avalancheup-aws --help
```

#### Option 2: Install `avalanche-ops` on Linux
```bash
rm -f /tmp/avalancheup-aws
wget "https://github.com/ava-labs/avalanche-ops/releases/download/latest/avalancheup-aws.x86_64-linux-gnu"
mv ./avalancheup-aws.x86_64-linux-gnu /tmp/avalancheup-aws
chmod +x /tmp/avalancheup-aws
/tmp/avalancheup-aws --help
```

### Step 2: Install and Configure `aws-cli`
Next, install the [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html). This is used to
manipulate CloudFormation.

Once you've installed the AWS CLI, run `aws configure` to set the access key to
use while deploying your devnet.

### Step 3: Install `token-cli`
```bash
export ARCH_TYPE=$(uname -m)
echo ${ARCH_TYPE}
export OS_TYPE=$(uname | tr '[:upper:]' '[:lower:]')
echo ${OS_TYPE}
export HYPERSDK_VERSION="0.0.5"
rm -f /tmp/token-cli
wget "https://github.com/ava-labs/hypersdk/releases/download/v${HYPERSDK_VERSION}/hypersdk_${HYPERSDK_VERSION}_${OS_TYPE}_${ARCH_TYPE}.tar.gz"
mkdir tmp-hypersdk
tar -xvf hypersdk_${HYPERSDK_VERSION}_${OS_TYPE}_${ARCH_TYPE}.tar.gz -C tmp-hypersdk
rm -rf hypersdk_${HYPERSDK_VERSION}_${OS_TYPE}_${ARCH_TYPE}.tar.gz
mv tmp-hypersdk/token-cli /tmp/token-cli
rm -rf tmp-hypersdk
```

### Step 4: Download `tokenvm`
`tokenvm` is currently compiled with `GOAMD=v1`. If you want to significantly
improve the performance of cryptographic operations, you should consider
building with [`v3+`](https://github.com/golang/go/wiki/MinimumRequirements#amd64).

```bash
export HYPERSDK_VERSION="0.0.5"
rm -f /tmp/tokenvm
wget "https://github.com/ava-labs/hypersdk/releases/download/v${HYPERSDK_VERSION}/hypersdk_${HYPERSDK_VERSION}_linux_amd64.tar.gz"
mkdir tmp-hypersdk
tar -xvf hypersdk_${HYPERSDK_VERSION}_linux_amd64.tar.gz -C tmp-hypersdk
rm -rf hypersdk_${HYPERSDK_VERSION}_linux_amd64.tar.gz
mv tmp-hypersdk/tokenvm /tmp/tokenvm
rm -rf tmp-hypersdk
```

*Note*: We must install the linux build because that is compatible with the AWS
deployment target. If you use Graivtron CPUs, make sure to use `arm64` here.


### Step 5: Plan Local Network Deploy

Now we can spin up a new network of 6 nodes with some defaults:
- `avalanche-ops` supports [Graviton-based processors](https://aws.amazon.com/ec2/graviton/) (ARM64). Use `--arch-type arm64` to run nodes in ARM64 CPUs.
- `avalanche-ops` supports [EC2 Spot instances](https://aws.amazon.com/ec2/spot/) for cost savings. Use `--instance-mode=spot` to run instances in spot mode.

```bash
/tmp/avalancheup-aws default-spec \
--arch-type amd64 \
--rust-os-type ubuntu20.04 \
--anchor-nodes 3 \
--non-anchor-nodes 3 \
--region us-west-2 \
--instance-mode=on-demand \
--instance-types=c5.4xlarge \
--ip-mode=elastic \
--metrics-fetch-interval-seconds 60 \
--network-name custom \
--keys-to-generate 5
```

The `default-spec` (and `apply`) command only provisions nodes, not Custom VMs.
The Subnet and Custom VM installation are done via `install-subnet-chain` sub-commands that follow.

#### Profiling `avalanchego`
If you'd like to profile `avalanchego`'s CPU and RAM, provide the following
flags to the command above:

```bash
avalancheup-aws default-spec \
...
--avalanchego-profile-continuous-enabled \
--avalanchego-profile-continuous-freq 1m \
--avalanchego-profile-continuous-max-files 10
```

#### Using Custom `avalanchego` Binaries
The default command above will download the `avalanchego` public release binaries from GitHub.
To test your own binaries, use the following flags to upload to S3. These binaries must be built
for the target remote machine platform (e.g., build for `aarch64` and Linux to run them in
Graviton processors):

```bash
avalancheup-aws default-spec \
...
--upload-artifacts-aws-volume-provisioner-local-bin ${AWS_VOLUME_PROVISIONER_BIN_PATH} \
--upload-artifacts-aws-ip-provisioner-local-bin ${AWS_IP_PROVISIONER_BIN_PATH} \
--upload-artifacts-avalanche-telemetry-cloudwatch-local-bin ${AVALANCHE_TELEMETRY_CLOUDWATCH_BIN_PATH} \
--upload-artifacts-avalanched-aws-local-bin ${AVALANCHED_AWS_BIN_PATH} \
--upload-artifacts-avalanchego-local-bin ${AVALANCHEGO_BIN_PATH} \
```

It is recommended to specify your own artifacts to avoid flaky github release page downloads.

#### Restricting Instance Types
By default, `avalanche-ops` will attempt to use all available instance types of
a given size (scoped by `--instance-size`) in a region in your cluster. If you'd like to restrict which
instance types are used (and override `--instance-size`), you can populate the `--instance-types` flag.
You can specify a single instance type (`--instance-types=c5.2xlarge`) or a comma-separated list
(`--instance-types=c5.2xlarge,m5.2xlarge`).

`avalanchego` will use at least `<expected block size> * (2048[bytesToIDCache] + 2048[decidedBlocksCache])`
for caching blocks. This overhead will be significantly reduced when
[this issue is resolved](https://github.com/ava-labs/hypersdk/issues/129).

#### Increasing Rate Limits
If you are attempting to stress test the Devnet, you should disable CPU, Disk,
and Bandwidth rate limiting. You can do this by adding the following lines to
`avalanchego_config` in the spec file:
```yaml
avalanchego_config:
    ...
    proposervm-use-current-height: true
    throttler-inbound-validator-alloc-size: 107374182
    throttler-inbound-node-max-processing-msgs: 100000
    throttler-inbound-bandwidth-refill-rate: 1073741824
    throttler-inbound-bandwidth-max-burst-size: 1073741824
    throttler-inbound-cpu-validator-alloc: 100000
    throttler-inbound-disk-validator-alloc: 10737418240000
    throttler-outbound-validator-alloc-size: 107374182
    snow-mixed-query-num-push-vdr-uint: 10
    consensus-on-accept-gossip-validator-size: 0
    consensus-on-accept-gossip-non-validator-size: 0
    consensus-on-accept-gossip-peer-size: 5
    consensus-accepted-frontier-gossip-peer-size: 5
    network-compression-enabled: false
```

#### Supporting All Metrics
By default, `avalanche-ops` does not support ingest all metrics into AWS
CloudWatch. To ingest all metrics, change the configured filters in
`upload_artifacts.prometheus_metrics_rules_file_path` to:
```yaml
filters:
  - regex: ^*$
```

### Step 6: Apply Local Network Deploy

The `default-spec` command itself does not create any resources, and instead outputs the following `apply` and `delete` commands that you can copy and paste:

```bash
# first make sure you have access to the AWS credential
aws sts get-caller-identity

# "avalancheup-aws apply" would not work...
# An error occurred (ExpiredToken) when calling the GetCallerIdentity operation: The security token included in the request is expired

# "avalancheup-aws apply" will work with the following credential
# {
#   "UserId": "...:abc@abc.com",
#   "Account": "...",
#   "Arn": "arn:aws:sts::....:assumed-role/name/abc@abc.com"
# }
```

```bash
# run the following to create resources
vi /home/ubuntu/aops-custom-****-***.yaml

avalancheup-aws apply \
--spec-file-path /home/ubuntu/aops-custom-****-***.yaml

# run the following to delete resources
/tmp/avalancheup-aws delete \
--delete-cloudwatch-log-group \
--delete-s3-objects \
--delete-ebs-volumes \
--delete-elastic-ips \
--spec-file-path /home/ubuntu/aops-custom-****-***.yaml
```

That is, `apply` creates AWS resources, whereas `delete` destroys after testing is done.

If you see the following command, that is expected (it means `avalanche-ops` is
reusing a S3 bucket it previously created):
```
[2023-03-23T23:49:01Z WARN  aws_manager::s3] bucket already exists so returning early (original error 'service error')
```

*SECURITY*: By default, the SSH and HTTP ports are open to public. Once you complete provisioning the nodes, go to EC2 security
group to restrict the inbound rules to your IP.

*NOTE*: In rare cases, you may encounter [aws-sdk-rust#611](https://github.com/awslabs/aws-sdk-rust/issues/611)
where AWS SDK call hangs, which blocks node bootstraps. If a node takes too long to start, connect to that
instance (e..g, use SSM sesson from your AWS console), and restart the agent with the command `sudo systemctl restart avalanched-aws`.

### Step 7: Generate Configs

Now that the network and nodes are up, let's install two subnets, each of which runs its own `tokenvm`. Use the following commands to
do so (similar to [scripts/run.sh](./scripts/run.sh)):

```bash
rm -rf /tmp/avalanche-ops
mkdir /tmp/avalanche-ops

cat <<EOF > /tmp/avalanche-ops/tokenvm-subnet-config.json
{
  "proposerMinBlockDelay": 0,
  "gossipOnAcceptPeerSize": 0,
  "gossipAcceptedFrontierPeerSize": 0
}
EOF
cat /tmp/avalanche-ops/tokenvm-subnet-config.json

cat <<EOF > /tmp/avalanche-ops/allocations.json
[{"address":"token1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97nsjzf3yp", "balance":1000000000000000}]
EOF

/tmp/token-cli genesis generate /tmp/avalanche-ops/allocations.json \
--genesis-file /tmp/avalanche-ops/tokenvm-genesis.json \
--max-block-units 4000000 \
--window-target-units 100000000000 \
--window-target-blocks 30
cat /tmp/avalanche-ops/tokenvm-genesis.json

cat <<EOF > /tmp/avalanche-ops/tokenvm-chain-config.json
{
  "mempoolSize": 10000000,
  "mempoolPayerSize": 10000000,
  "mempoolExemptPayers":["token1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97nsjzf3yp"],
  "streamingBacklogSize": 10000000,
  "trackedPairs":["*"],
  "logLevel": "info",
  "preferredBlocksPerSecond": 3,
  "decisionsPort": 9652,
  "blocksPort": 9653
}
EOF
cat /tmp/avalanche-ops/tokenvm-chain-config.json
```

*Note*: Make sure that port `9652` and `9653` are open on the AWS Security Group applied to all
nodes otherwise the `token-cli` will not work properly. This requirement will
be removed when the [HyperSDK migrates to using proper
WebSockets](https://github.com/ava-labs/hypersdk/issues/64).

#### Profiling `tokenvm`
If you'd like to profile `tokenvm`'s CPU and RAM, add the following line to the
`tokenvm-chain-config.json` file above:

```json
{
  ...
  "continuousProfilerDir": "/var/log/tokenvm-profile"
}
```

### Step 8: Install Chains
You can run the following commands to spin up 2 `tokenvm` Devnets. Make sure to
replace the `***` fields, IP addresses, key, and `node-ids-to-instance-ids` with your own data:
```bash
/tmp/avalancheup-aws install-subnet-chain \
--log-level info \
--region us-west-2 \
--s3-bucket <TODO> \
--s3-key-prefix <TODO> \
--ssm-doc <TODO> \
--chain-rpc-url <TODO> \
--key <TODO> \
--primary-network-validate-period-in-days 16 \
--subnet-validate-period-in-days 14 \
--subnet-config-local-path /tmp/avalanche-ops/tokenvm-subnet-config.json \
--subnet-config-remote-dir /data/avalanche-configs/subnets \
--vm-binary-local-path /tmp/tokenvm \
--vm-binary-remote-dir /data/avalanche-plugins \
--chain-name tokenvm1 \
--chain-genesis-path /tmp/avalanche-ops/tokenvm-genesis.json \
--chain-config-local-path /tmp/avalanche-ops/tokenvm-chain-config.json \
--chain-config-remote-dir /data/avalanche-configs/chains \
--avalanchego-config-remote-path /data/avalanche-configs/config.json \
--node-ids-to-instance-ids <TODO>
```

#### Viewing Logs
1) Open the [AWS CloudWatch](https://aws.amazon.com/cloudwatch) product on your
AWS Console
2) Click "Logs Insights" on the left pane
3) Use the following query to view all logs (in reverse-chronological order)
for all nodes in your Devnet:
```
fields @timestamp, @message, @logStream, @log
| filter(@logStream not like "avalanche-telemetry-cloudwatch.log")
| filter(@logStream not like "syslog")
| sort @timestamp desc
| limit 20
```

*Note*: The "Log Group" you are asked to select should have a similar name as
the spec file that was output earlier.

#### Viewing Metrics
#### Option 1: AWS CloudWatch
1) Open the [AWS CloudWatch](https://aws.amazon.com/cloudwatch) product on your
AWS Console
2) Click "All Metrics" on the left pane
3) Click the "Custom Namespace" that matches the name of the spec file

#### Option 2: Prometheus
To view metrics, first download and install [Prometheus](https://prometheus.io/download/)
using the following commands:
```bash
rm -f /tmp/prometheus
wget https://github.com/prometheus/prometheus/releases/download/v2.43.0/prometheus-2.43.0.darwin-amd64.tar.gz
tar -xvf prometheus-2.43.0.darwin-amd64.tar.gz
rm prometheus-2.43.0.darwin-amd64.tar.gz
mv prometheus-2.43.0.darwin-amd64/prometheus /tmp/prometheus
rm -rf prometheus-2.43.0.darwin-amd64
```

Once you have Prometheus installed, run the following command to auto-generate
a configuration file:
```bash
/tmp/token-cli metrics prometheus <avalanche-ops spec file path> /tmp/prometheus.yaml
```

In a separate terminal, then run the following command to view collected
metrics:
```bash
/tmp/prometheus --config.file=/tmp/prometheus.yaml
```

Here are some useful queries (on an example chainID `3rihqpXh6ZJqxL2dsrVysKkEKroiD9tvQWLS6iWVnd8K4HST2`):
* `avalanche_3rihqpXh6ZJqxL2dsrVysKkEKroiD9tvQWLS6iWVnd8K4HST2_blks_processing`
* `avalanche_3rihqpXh6ZJqxL2dsrVysKkEKroiD9tvQWLS6iWVnd8K4HST2_blks_rejected_count`
* `avalanche_3rihqpXh6ZJqxL2dsrVysKkEKroiD9tvQWLS6iWVnd8K4HST2_blks_accepted_count`
* [verify latency in ms] `increase(avalanche_3rihqpXh6ZJqxL2dsrVysKkEKroiD9tvQWLS6iWVnd8K4HST2_vm_metervm_verify_sum[30s:1s])/increase(avalanche_3rihqpXh6ZJqxL2dsrVysKkEKroiD9tvQWLS6iWVnd8K4HST2_vm_metervm_verify_count[30s:1s])/1000000`
* [accept latency in ms] `increase(avalanche_3rihqpXh6ZJqxL2dsrVysKkEKroiD9tvQWLS6iWVnd8K4HST2_vm_metervm_accept_sum[30s:1s])/increase(avalanche_3rihqpXh6ZJqxL2dsrVysKkEKroiD9tvQWLS6iWVnd8K4HST2_vm_metervm_accept_count[30s:1s])/1000000`
* [transactions per second] `deriv(avalanche_3rihqpXh6ZJqxL2dsrVysKkEKroiD9tvQWLS6iWVnd8K4HST2_vm_hyper_sdk_vm_txs_accepted[30s:1s])`
* [accepted block size] `increase(avalanche_3rihqpXh6ZJqxL2dsrVysKkEKroiD9tvQWLS6iWVnd8K4HST2_blks_accepted_container_size_sum[30s:1s])/increase(avalanche_3rihqpXh6ZJqxL2dsrVysKkEKroiD9tvQWLS6iWVnd8K4HST2_blks_accepted_count[30s:1s])`
* `avalanche_3rihqpXh6ZJqxL2dsrVysKkEKroiD9tvQWLS6iWVnd8K4HST2_vm_hyper_sdk_vm_txs_accepted`
* [disk used] `300000000000-avalanche_resource_tracker_disk_available_space`
* `avalanche_resource_tracker_cpu_usage`
* `avalanche_resource_tracker_disk_reads`
* `avalanche_resource_tracker_disk_writes`
* `avalanche_3rihqpXh6ZJqxL2dsrVysKkEKroiD9tvQWLS6iWVnd8K4HST2_vm_go_goroutines`
* `avalanche_3rihqpXh6ZJqxL2dsrVysKkEKroiD9tvQWLS6iWVnd8K4HST2_vm_go_memstats_alloc_bytes`
* `avalanche_network_inbound_conn_throttler_rate_limited`

To estimate how many `ms/s` the consensus engine and `tokenvm` are spending busy, use the
following query (on an example chainID `2CziMHmCB6obvfaXjstLQAzWy2HJZ2HGj3c8jiaDG16teaeL2X`):
```
increase(avalanche_2CziMHmCB6obvfaXjstLQAzWy2HJZ2HGj3c8jiaDG16teaeL2X_handler_chits_sum[30s])/1000000/30 + increase(avalanche_2CziMHmCB6obvfaXjstLQAzWy2HJZ2HGj3c8jiaDG16teaeL2X_handler_notify_sum[30s])/1000000/30 + increase(avalanche_2CziMHmCB6obvfaXjstLQAzWy2HJZ2HGj3c8jiaDG16teaeL2X_handler_get_sum[30s])/1000000/30 + increase(avalanche_2CziMHmCB6obvfaXjstLQAzWy2HJZ2HGj3c8jiaDG16teaeL2X_handler_push_query_sum[30s])/1000000/30 + increase(avalanche_2CziMHmCB6obvfaXjstLQAzWy2HJZ2HGj3c8jiaDG16teaeL2X_handler_put_sum[30s])/1000000/30 + increase(avalanche_2CziMHmCB6obvfaXjstLQAzWy2HJZ2HGj3c8jiaDG16teaeL2X_handler_pull_query_sum[30s])/1000000/30 + increase(avalanche_2CziMHmCB6obvfaXjstLQAzWy2HJZ2HGj3c8jiaDG16teaeL2X_handler_query_failed_sum[30s])/1000000/30
```

To isolate just how many `ms/s` the consensus engine is spending busy (removing
"build", "verify", and "accept" time spent in `tokenvm`), use the following
command:
```
increase(avalanche_2CziMHmCB6obvfaXjstLQAzWy2HJZ2HGj3c8jiaDG16teaeL2X_handler_chits_sum[30s])/1000000/30 + increase(avalanche_2CziMHmCB6obvfaXjstLQAzWy2HJZ2HGj3c8jiaDG16teaeL2X_handler_notify_sum[30s])/1000000/30 + increase(avalanche_2CziMHmCB6obvfaXjstLQAzWy2HJZ2HGj3c8jiaDG16teaeL2X_handler_get_sum[30s])/1000000/30 + increase(avalanche_2CziMHmCB6obvfaXjstLQAzWy2HJZ2HGj3c8jiaDG16teaeL2X_handler_push_query_sum[30s])/1000000/30 + increase(avalanche_2CziMHmCB6obvfaXjstLQAzWy2HJZ2HGj3c8jiaDG16teaeL2X_handler_put_sum[30s])/1000000/30 + increase(avalanche_2CziMHmCB6obvfaXjstLQAzWy2HJZ2HGj3c8jiaDG16teaeL2X_handler_pull_query_sum[30s])/1000000/30 + increase(avalanche_2CziMHmCB6obvfaXjstLQAzWy2HJZ2HGj3c8jiaDG16teaeL2X_handler_query_failed_sum[30s])/1000000/30 - increase(avalanche_2CziMHmCB6obvfaXjstLQAzWy2HJZ2HGj3c8jiaDG16teaeL2X_vm_metervm_build_block_sum[30s])/1000000/30 - increase(avalanche_2CziMHmCB6obvfaXjstLQAzWy2HJZ2HGj3c8jiaDG16teaeL2X_vm_metervm_verify_sum[30s])/1000000/30 - increase(avalanche_2CziMHmCB6obvfaXjstLQAzWy2HJZ2HGj3c8jiaDG16teaeL2X_vm_metervm_accept_sum[30s])/1000000/30
```

To remove previously ingested data, delete for a folder called `data` in the
directory where you last ran Prometheus.

### Step 9: Initialize `token-cli`
You can import the demo key and the network configuration from `avalanche-ops`
using the following commands:
```bash
/tmp/token-cli key import demo.pk
/tmp/token-cli chain import-ops <chainID> <avalanche-ops spec file path>
/tmp/token-cli key balance --check-all-chains
```

If you previously used `token-cli`, you may want to delete data you previously
ingested into it. You can do this with the following command:
```bash
rm -rf .token-cli
```

### Step 10: Start Integrated Block Explorer
To view activity on each Subnet you created, you can run the `token-cli`'s
integrated block explorer. To do this, run the following command:
```bash
/tmp/token-cli chain watch --hide-txs
```

If you don't plan to load test the devnet, you may wush to just run the
following command to get additional transaction details:
```bash
/tmp/token-cli chain watch
```

### Step 11: Run Load
Once the network information is imported, you can then run the following
command to drive an arbitrary amount of load:
```bash
/tmp/token-cli spam run
```

#### Limiting Inflight Txs
To ensure you don't create too large of a backlog on the network, you can add
run the following command (defaults to `72000`):
```bash
/tmp/token-cli spam run --max-tx-backlog 5000
```

### [OPTIONAL] Step 12: SSH Into Nodes
You can SSH into any machine created by `avalanche-ops` using the SSH key
automatically generated during the `apply` command. The commands for doing so
are emitted during `apply` and look something like this:
```bash
ssh -o "StrictHostKeyChecking no" -i aops-custom-202303-21qJUU-ec2-access.key ubuntu@34.209.76.123
```

### [OPTIONAL] Step 13: Deploy Another Subnet
To test Avalanche Warp Messaging, you must be running at least 2 Subnets. To do
so, just replicate the command you ran above with a different `--chain-name` (and
a different set of validators):
```bash
/tmp/avalancheup-aws install-subnet-chain \
--log-level info \
--region us-west-2 \
--s3-bucket <TODO> \
--s3-key-prefix <TODO> \
--ssm-doc <TODO> \
--chain-rpc-url <TODO> \
--key <TODO> \
--primary-network-validate-period-in-days 16 \
--subnet-validate-period-in-days 14 \
--subnet-config-local-path /tmp/avalanche-ops/tokenvm-subnet-config.json \
--subnet-config-remote-dir /data/avalanche-configs/subnets \
--vm-binary-local-path /tmp/tokenvm \
--vm-binary-remote-dir /data/avalanche-plugins \
--chain-name tokenvm2 \
--chain-genesis-path /tmp/avalanche-ops/tokenvm-genesis.json \
--chain-config-local-path /tmp/avalanche-ops/tokenvm-chain-config.json \
--chain-config-remote-dir /data/avalanche-configs/chains \
--avalanchego-config-remote-path /data/avalanche-configs/config.json \
--node-ids-to-instance-ids <TODO>
```

## Deploy TokenVM in Fuji network with avalanche-ops
Same as above, except you use `--network-name fuji` and do not need anchor nodes:

```bash
avalancheup-aws default-spec \
--arch-type amd64 \
--rust-os-type ubuntu20.04 \
--non-anchor-nodes 6 \
--region us-west-2 \
--instance-mode=on-demand \
--instance-size=2xlarge \
--ip-mode=elastic \
--metrics-fetch-interval-seconds 60 \
--network-name fuji
```

Make sure the nodes are in sync with the chain state before installing
subnets/chains with `avalancheup-aws install-subnet-chain`. You can check the status
of the nodes either via HTTP `/health` endpoints or CloudWatch logs.
