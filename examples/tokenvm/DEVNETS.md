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
export ARCH_TYPE=$(uname -m)
echo ${ARCH_TYPE}
rm -f /tmp/avalancheup-aws
wget "https://github.com/ava-labs/avalanche-ops/releases/download/latest/avalancheup-aws.${ARCH_TYPE}-apple-darwin"
mv ./avalancheup-aws.${ARCH_TYPE}-apple-darwin /tmp/avalancheup-aws
chmod +x /tmp/avalancheup-aws
/tmp/avalancheup-aws --help
```

#### Option 2: Install `avalanche-ops` on Linux
```bash
export ARCH_TYPE=$(uname -m)
echo ${ARCH_TYPE}
rm -f /tmp/avalancheup-aws
wget "https://github.com/ava-labs/avalanche-ops/releases/download/latest/avalancheup-aws.${ARCH_TYPE}-linux-gnu"
mv ./avalancheup-aws.${ARCH_TYPE}-linux-gnu /tmp/avalancheup-aws
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
export HYPERSDK_VERSION="0.0.4"
rm -f ./tmp/token-cli
wget "https://github.com/ava-labs/hypersdk/releases/download/v${HYPERSDK_VERSION}/hypersdk_${HYPERSDK_VERSION}_${OS_TYPE}_${ARCH_TYPE}.tar.gz
tar -xvf hypersdk_${HYPERSDK_VERSION}_${OS_TYPE}_${ARCH_TYPE}.tar.gz
mv hypersdk_${HYPERSDK_VERSION}_${OS_TYPE}_${ARCH_TYPE}/token-cli /tmp/token-cli
```

### Step 4: Download `tokenvm`
`tokenvm` is currently compiled with `GOAMD=v1`. If you want to significantly
improve the performance of cryptographic operations, you should consider
building with [`v3+`](https://github.com/golang/go/wiki/MinimumRequirements#amd64).

```bash
export HYPERSDK_VERSION="0.0.4"
rm -f ./tmp/tokenvm
wget "https://github.com/ava-labs/hypersdk/releases/download/v${HYPERSDK_VERSION}/hypersdk_${HYPERSDK_VERSION}_linux_amd64.tar.gz
tar -xvf hypersdk_${HYPERSDK_VERSION}_linux_amd64.tar.gz
mv hypersdk_${HYPERSDK_VERSION}_linux_amd64/tokenvm /tmp/tokenvm
```

*Note*: We must install the linux build because that is compatible with the AWS
deployment target. If you use Graivtron CPUs, make sure to use `arm64` here.


### Step 5: Plan Local Network Deploy

Now we can spin up a new network of 6 nodes with some defaults:
- `avalanche-ops` supports [Graviton-based processors](https://aws.amazon.com/ec2/graviton/) (ARM64). Use `--arch-type arm64` to run nodes in ARM64 CPUs.
- `avalanche-ops` supports [EC2 Spot instances](https://aws.amazon.com/ec2/spot/) for cost savings. Use `--instance-mode=spot` to run instances in spot mode.

```bash
# launch/download all artifacts for x86 platform
/tmp/avalancheup-aws default-spec \
--arch-type amd64 \
--rust-os-type ubuntu20.04 \
--anchor-nodes 3 \
--non-anchor-nodes 3 \
--region us-west-2 \
--instance-mode=on-demand \
--instance-size=2xlarge \
--ip-mode=elastic \
--metrics-fetch-interval-seconds 60 \
--network-name custom \
--keys-to-generate 5
```

The `default-spec` (and `apply`) command only provisions nodes, not Custom VMs.
The Subnet and Custom VM installation are done via `install-subnet-chain` sub-commands that follow.

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
By default, `avalanche-ops` will attempt to use all available instance types in
a region in your cluster. If you'd like to restrict which instance types are used,
edit the output `aops-custom-****-***.yaml` file before deploying:
```yaml
machine:
  anchor_nodes: 3
  non_anchor_nodes: 3
  arch_type: amd64
  rust_os_type: ubuntu20.04
  instance_types:
  - t3a.2xlarge
  - t3.2xlarge
  - t2.2xlarge
  - c6a.2xlarge
  - m6a.2xlarge
  - m5.2xlarge
  - c5.2xlarge
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
avalancheup-aws delete \
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

avalancheup-aws subnet-config \
--log-level=info \
--proposer-min-block-delay 0 \
--file-path /tmp/avalanche-ops/subnet-config.json

cat <<EOF > /tmp/avalanche-ops/allocations.json
[{"address":"token1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97nsjzf3yp", "balance":1000000000000}]
EOF
rm -f /tmp/avalanche-ops/tokenvm-genesis.json
/tmp/token-cli genesis generate /tmp/avalanche-ops/allocations.json \
--genesis-file /tmp/avalanche-ops/tokenvm-genesis.json
cat /tmp/avalanche-ops/tokenvm-genesis.json

cat <<EOF > /tmp/avalanche-ops/tokenvm-chain-config.json
{
  "mempoolSize": 10000000,
  "mempoolPayerSize": 10000000,
  "mempoolExemptPayers":["token1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97nsjzf3yp"],
  "streamingBacklogSize": 10000000,
  "trackedPairs":["*"],
  "logLevel": "info"
}
EOF
cat /tmp/avalanche-ops/tokenvm-chain-config.json
```

### Step 8: Install Chains
You can run the following commands to spin up 2 `tokenvm` Devnets. Make sure to
replace the `***` fields, IP addresses, key, and `node-ids-to-instance-ids` with your own data:

```bash
/tmp/avalancheup-aws install-subnet-chain \
--log-level info \
--region us-west-2 \
--s3-bucket avalanche-ops-***-us-west-2 \
--s3-key-prefix aops-custom-***/install-subnet-chain \
--ssm-doc aops-custom-***-ssm-install-subnet-chain \
--chain-rpc-url http://52.88.8.107:9650 \
--key 0x56289e99c94b6912bfc12adc093c9b51124f0dc54ac7a766b2bc5ccf558d8027 \
--primary-network-validate-period-in-days 16 \
--subnet-validate-period-in-days 14 \
--subnet-config-local-path /tmp/avalanche-ops/subnet-config.json \
--subnet-config-remote-dir /data/avalanche-configs/subnets \
--vm-binary-local-path /tmp/avalanche-ops/tokenvm \
--vm-binary-remote-dir /data/avalanche-plugins \
--chain-name tokenvm1 \
--chain-genesis-path /tmp/avalanche-ops/tokenvm-genesis.json \
--chain-config-local-path /tmp/avalanche-ops/tokenvm-chain-config.json \
--chain-config-remote-dir /data/avalanche-configs/chains \
--avalanchego-config-remote-path /data/avalanche-configs/config.json \
--node-ids-to-instance-ids '{"NodeID-31HAxw7wYJ2u2HQHBkwwF26bnFuxnh2sa":"i-04f6ea239218440f0","NodeID-PLM2si9LWaqvzid6AeJa9tft7rzDXXKg2":"i-0459af0e4cf31fcfa","NodeID-D1RtuFSbmcTiRVY991vkq5UvsTBUpHHLR":"i-0d44cfe5420370c30"}'
#  VM Id 'tHBYNu8ikt25R77fH4znHYC4B5mkaEnXPFmsJnECZjq59dySw', chain name 'tokenvm1'
# SUCCESS: subnet Id 2DLqm2Wk4SFtdmeqkmyfAvTdfvNKmbjwZwztMqRaQCwnDbRHCo, blockchain Id G9CbuiKLoyeYhabA8ph7TBiisKt9LS6Hx1QRgoajnwxd1xFC8

# this will prompt you to confirm the outcome!
# so make sure to check the outputs!
/tmp/avalancheup-aws install-subnet-chain \
--log-level info \
--region us-west-2 \
--s3-bucket avalanche-ops-***-us-west-2 \
--s3-key-prefix aops-custom-***/install-subnet-chain \
--ssm-doc aops-custom-***-ssm-install-subnet-chain \
--chain-rpc-url http://52.88.8.107:9650 \
--key 0x56289e99c94b6912bfc12adc093c9b51124f0dc54ac7a766b2bc5ccf558d8027 \
--primary-network-validate-period-in-days 16 \
--subnet-validate-period-in-days 14 \
--subnet-config-local-path /tmp/avalanche-ops/subnet-config.json \
--subnet-config-remote-dir /data/avalanche-configs/subnets \
--vm-binary-local-path /tmp/avalanche-ops/tokenvm \
--vm-binary-remote-dir /data/avalanche-plugins \
--chain-name tokenvm2 \
--chain-genesis-path /tmp/avalanche-ops/tokenvm-genesis.json \
--chain-config-local-path /tmp/avalanche-ops/tokenvm-chain-config.json \
--chain-config-remote-dir /data/avalanche-configs/chains \
--avalanchego-config-remote-path /data/avalanche-configs/config.json \
--node-ids-to-instance-ids '{"NodeID-PNc5fwhKLDLGHBF81qnjTwsdjFbdcZxn1":"i-0c5114f6b1e48e671","NodeID-8oqy47xm46RcdTNFxYo8dpBJtPFqQLLeG":"i-083ccebe7b8d40bc9","NodeID-HC9HVTiThxL6d55bNGqV3bkKPwQmLvEs1":"i-0806a42d5bb597dc5"}'
# VM Id 'tHBYNu8ikt4i8cEV4nsSuj7Ldc9sXAHc8L6qKRJR4e5CR7T3t', chain name 'tokenvm2'
# SUCCESS: subnet Id 2km1PAKNK4vSfpyjgK2iFFoBFD3mXHBAWfDBVsPuzKtNYdPRTY, blockchain Id F1c3GayrV5E7NQdxUCbVfnRPsMSxGbTuHvJLS4R48M3Dcazs6
```

*Note*: `--chain-name tokenvm*` and `--node-ids-to-instance-ids` values are different to split nodes
into two separate subnets. And `avalancheup-aws install-subnet-chain` **will add the specified nodes
as primary network validators (regardless of it's for customer or public networks) before adding them
as subnet validators**. So, please make sure the `--key` has enough balance and use
`--primary-network-validate-period-in-days` and `--subnet-validate-period-in-days` flags to set custom
validate priods (defaults to 16-day for primary network, 14-day for subnet validation).

### Step 9: Check Healthiness
To make sure chains are launched successfully, query the `health` endpoints:

> curl http://52.88.8.107:9650/ext/health
>
> {"checks":{"C":{"message":{"consensus":{"longestRunningBlock":"0s","outstandingBlocks":0},"vm":null},"timestamp":"2023-03-14T15:55:08.256378577Z","duration":14020},"F1c3GayrV5E7NQdxUCbVfnRPsMSxGbTuHvJLS4R48M3Dcazs6":{"message":{"consensus":{"longestRunningBlock":"0s","outstandingBlocks":0},"vm":{"database":{"v1.4.5":null},"health":200}},"timestamp":"2023-03-14T15:55:08.257445968Z","duration":1061711},"P":{"message":{"consensus":{"longestRunningBlock":"0s","outstandingBlocks":0},"vm":{"2km1PAKNK4vSfpyjgK2iFFoBFD3mXHBAWfDBVsPuzKtNYdPRTY-percentConnected":1,"primary-percentConnected":1}},"timestamp":"2023-03-14T15:55:08.256454548Z","duration":74871},"X":{"message":{"consensus":{"outstandingVertices":0,"snowstorm":{"outstandingTransactions":0}},"vm":null},"timestamp":"2023-03-14T15:55:08.256459848Z","duration":15850},"bootstrapped":{"message":[],"timestamp":"2023-03-14T15:55:08.256438778Z","duration":6020},"database":{"timestamp":"2023-03-14T15:55:08.256464299Z","duration":1420},"diskspace":{"message":{"availableDiskBytes":299770843136},"timestamp":"2023-03-14T15:55:08.256466309Z","duration":4070},"network":{"message":{"connectedPeers":5,"sendFailRate":0,"timeSinceLastMsgReceived":"256.459548ms","timeSinceLastMsgSent":"256.459548ms"},"timestamp":"2023-03-14T15:55:08.256461929Z","duration":5551},"router":{"message":{"longestRunningRequest":"0s","outstandingRequests":0},"timestamp":"2023-03-14T15:55:08.256363507Z","duration":17231}},"healthy":true}

> curl http://44.224.148.127:9650/ext/health
>
> {"checks":{"C":{"message":{"consensus":{"longestRunningBlock":"0s","outstandingBlocks":0},"vm":null},"timestamp":"2023-03-14T15:55:08.203325635Z","duration":6570},"F1c3GayrV5E7NQdxUCbVfnRPsMSxGbTuHvJLS4R48M3Dcazs6":{"message":{"consensus":{"longestRunningBlock":"0s","outstandingBlocks":0},"vm":{"database":{"v1.4.5":null},"health":200}},"timestamp":"2023-03-14T15:55:08.204161784Z","duration":927270},"P":{"message":{"consensus":{"longestRunningBlock":"0s","outstandingBlocks":0},"vm":{"2km1PAKNK4vSfpyjgK2iFFoBFD3mXHBAWfDBVsPuzKtNYdPRTY-percentConnected":1,"primary-percentConnected":1}},"timestamp":"2023-03-14T15:55:08.203318015Z","duration":63060},"X":{"message":{"consensus":{"outstandingVertices":0,"snowstorm":{"outstandingTransactions":0}},"vm":null},"timestamp":"2023-03-14T15:55:08.203233714Z","duration":18360},"bootstrapped":{"message":[],"timestamp":"2023-03-14T15:55:08.203205494Z","duration":3010},"database":{"timestamp":"2023-03-14T15:55:08.203214634Z","duration":1350},"diskspace":{"message":{"availableDiskBytes":299770839040},"timestamp":"2023-03-14T15:55:08.203252895Z","duration":5361},"network":{"message":{"connectedPeers":5,"sendFailRate":0,"timeSinceLastMsgReceived":"203.209294ms","timeSinceLastMsgSent":"203.209294ms"},"timestamp":"2023-03-14T15:55:08.203212714Z","duration":6550},"router":{"message":{"longestRunningRequest":"0s","outstandingRequests":0},"timestamp":"2023-03-14T15:55:08.203338225Z","duration":21220}},"healthy":true}

### Step 10: Run Spam
You can import the demo key and the network configuration from `avalanche-ops`
using the following commands:
```bash
/tmp/token-cli key import demo.pk
/tmp/token-cli chain import-ops
```

Once the network information is imported, you can then run `/tmp/token-cli spam
run` to push network activity on the devnet.

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
