## Deploy TokenVM in custom network with avalanche-ops

[avalanche-ops](https://github.com/ava-labs/avalanche-ops) provides a command-line interface to set up nodes and install custom VMs with the support for custom networks.

### Step 1

Install [Rust](https://www.rust-lang.org/tools/install) to compile avalanche-ops, or download from the [latest release page](https://github.com/ava-labs/avalanche-ops/releases/tag/latest):

```bash
git clone git@github.com:ava-labs/avalanche-ops.git
cd ./avalanche-ops
./scripts/build.release.sh
./target/release/avalancheup-aws --help
```

```bash
# to download on Mac
export LINUX_ARCH_TYPE=$(uname -m)
echo ${LINUX_ARCH_TYPE}
rm -f ./avalancheup-aws.${LINUX_ARCH_TYPE}-apple-darwin
wget "https://github.com/ava-labs/avalanche-ops/releases/download/latest/avalancheup-aws.${LINUX_ARCH_TYPE}-apple-darwin"
mv ./avalancheup-aws.${LINUX_ARCH_TYPE}-apple-darwin /tmp/avalancheup-aws
chmod +x /tmp/avalancheup-aws
/tmp/avalancheup-aws --help
```

```bash
# to download on Linux
export LINUX_ARCH_TYPE=$(uname -m)
echo ${LINUX_ARCH_TYPE}
rm -f ./avalancheup-aws.${LINUX_ARCH_TYPE}-linux-gnu
wget "https://github.com/ava-labs/avalanche-ops/releases/download/latest/avalancheup-aws.${LINUX_ARCH_TYPE}-linux-gnu"
mv ./avalancheup-aws.${LINUX_ARCH_TYPE}-linux-gnu /tmp/avalancheup-aws
chmod +x /tmp/avalancheup-aws
/tmp/avalancheup-aws --help
```

### Step 2

Now we can spin up a new network of 6 nodes with some defaults:

```bash
avalancheup-aws default-spec \
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

### Step 3

The `default-spec` command above will output `apply` and `delete` commands that you can copy and paste:

```bash
# run the following to create resources
vi /home/ubuntu/aops-custom-202303-vzLY7T.yaml

/home/ubuntu/avalanche-ops/target/release/avalancheup-aws apply \
--spec-file-path /home/ubuntu/aops-custom-202303-vzLY7T.yaml

# run the following to delete resources
/home/ubuntu/avalanche-ops/target/release/avalancheup-aws delete \
--delete-cloudwatch-log-group \
--delete-s3-objects \
--delete-ebs-volumes \
--delete-elastic-ips \
--spec-file-path /home/ubuntu/aops-custom-202303-vzLY7T.yaml
```

`apply` creates AWS resources, whereas `delete` destroys after testing is done.

NOTE: You may encounter [aws-sdk-rust#611](https://github.com/awslabs/aws-sdk-rust/issues/611) where AWS SDK call hangs, which blocks node bootstraps. If a node takes too long to start, connect to that instance (use SSM sesson from AWS console), and restart the agent with the command `sudo systemctl restart avalanched-aws`.

### Step 4

Now that the network and nodes are up, we can install two subnets, each of which runs its own TokenVM. A successful `avalancheup-aws apply` run will output the commands as follow (which you can customize depending on your VM):

```bash
# EXAMPLE: write subnet config
/home/ubuntu/avalanche-ops/target/release/avalancheup-aws subnet-config \
--log-level=info \
--proposer-min-block-delay 250000000 \
--file-path /tmp/subnet-config.json

# EXAMPLE: install subnet-evm in all nodes
/home/ubuntu/avalanche-ops/target/release/avalancheup-aws install-subnet-chain \
--log-level info \
--region us-west-2 \
--s3-bucket avalanche-ops-202303-3hxfve9pfh-us-west-2 \
--s3-key-prefix aops-custom-202303-2qUWt5/install-subnet-chain \
--ssm-doc aops-custom-202303-2qUWt5-ssm-install-subnet-chain \
--chain-rpc-url http://52.88.8.107:9650 \
...
```

For instance, use the following commands to deploy two TokenVM subnets (similar to [scripts/run.sh](./scripts/run.sh)):

```bash
/home/ubuntu/avalanche-ops/target/release/avalancheup-aws subnet-config \
--log-level=info \
--proposer-min-block-delay 0 \
--file-path /tmp/subnet-config.json

cat <<EOF > /tmp/allocations.json
[{"address":"token1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97nsjzf3yp", "balance":1000000000000}]
EOF
rm -f /tmp/tokenvm-genesis.json
/tmp/token-cli genesis generate /tmp/allocations.json \
--genesis-file /tmp/tokenvm-genesis.json
cat /tmp/tokenvm-genesis.json

cat <<EOF > /tmp/tokenvm-chain-config.json
{
  "mempoolSize": 10000000,
  "mempoolExemptPayers":["token1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97nsjzf3yp"],
  "parallelism": 5,
  "trackedPairs":["*"],
  "logLevel": "info",
  "stateSyncServerDelay": 0
}
EOF
cat /tmp/tokenvm-chain-config.json
```

```bash
/home/ubuntu/avalanche-ops/target/release/avalancheup-aws install-subnet-chain \
--log-level info \
--region us-west-2 \
--s3-bucket avalanche-ops-202303-3hxfve9pfh-us-west-2 \
--s3-key-prefix aops-custom-202303-2qUWt5/install-subnet-chain \
--ssm-doc aops-custom-202303-2qUWt5-ssm-install-subnet-chain \
--chain-rpc-url http://52.88.8.107:9650 \
--key 0x56289e99c94b6912bfc12adc093c9b51124f0dc54ac7a766b2bc5ccf558d8027 \
--staking-perioid-in-days 15 \
--subnet-config-local-path /tmp/subnet-config.json \
--subnet-config-remote-dir /data/avalanche-configs/subnets \
--vm-binary-local-path /tmp/tokenvm \
--vm-binary-remote-dir /data/avalanche-plugins \
--chain-name tokenvm1 \
--chain-genesis-path /tmp/tokenvm-genesis.json \
--chain-config-local-path /tmp/tokenvm-chain-config.json \
--chain-config-remote-dir /data/avalanche-configs/chains \
--avalanchego-config-remote-path /data/avalanche-configs/config.json \
--node-ids-to-instance-ids '{"NodeID-31HAxw7wYJ2u2HQHBkwwF26bnFuxnh2sa":"i-04f6ea239218440f0","NodeID-PLM2si9LWaqvzid6AeJa9tft7rzDXXKg2":"i-0459af0e4cf31fcfa","NodeID-D1RtuFSbmcTiRVY991vkq5UvsTBUpHHLR":"i-0d44cfe5420370c30"}'
#  VM Id 'tHBYNu8ikt25R77fH4znHYC4B5mkaEnXPFmsJnECZjq59dySw', chain name 'tokenvm1'
# SUCCESS: subnet Id 2DLqm2Wk4SFtdmeqkmyfAvTdfvNKmbjwZwztMqRaQCwnDbRHCo, blockchain Id G9CbuiKLoyeYhabA8ph7TBiisKt9LS6Hx1QRgoajnwxd1xFC8

/home/ubuntu/avalanche-ops/target/release/avalancheup-aws install-subnet-chain \
--log-level info \
--region us-west-2 \
--s3-bucket avalanche-ops-202303-3hxfve9pfh-us-west-2 \
--s3-key-prefix aops-custom-202303-2qUWt5/install-subnet-chain \
--ssm-doc aops-custom-202303-2qUWt5-ssm-install-subnet-chain \
--chain-rpc-url http://52.88.8.107:9650 \
--key 0x56289e99c94b6912bfc12adc093c9b51124f0dc54ac7a766b2bc5ccf558d8027 \
--staking-perioid-in-days 15 \
--subnet-config-local-path /tmp/subnet-config.json \
--subnet-config-remote-dir /data/avalanche-configs/subnets \
--vm-binary-local-path /tmp/tokenvm \
--vm-binary-remote-dir /data/avalanche-plugins \
--chain-name tokenvm2 \
--chain-genesis-path /tmp/tokenvm-genesis.json \
--chain-config-local-path /tmp/tokenvm-chain-config.json \
--chain-config-remote-dir /data/avalanche-configs/chains \
--avalanchego-config-remote-path /data/avalanche-configs/config.json \
--node-ids-to-instance-ids '{"NodeID-PNc5fwhKLDLGHBF81qnjTwsdjFbdcZxn1":"i-0c5114f6b1e48e671","NodeID-8oqy47xm46RcdTNFxYo8dpBJtPFqQLLeG":"i-083ccebe7b8d40bc9","NodeID-HC9HVTiThxL6d55bNGqV3bkKPwQmLvEs1":"i-0806a42d5bb597dc5"}'
# VM Id 'tHBYNu8ikt4i8cEV4nsSuj7Ldc9sXAHc8L6qKRJR4e5CR7T3t', chain name 'tokenvm2'
# SUCCESS: subnet Id 2km1PAKNK4vSfpyjgK2iFFoBFD3mXHBAWfDBVsPuzKtNYdPRTY, blockchain Id F1c3GayrV5E7NQdxUCbVfnRPsMSxGbTuHvJLS4R48M3Dcazs6
```

To make sure chains are launched successfully, just check the `health` endpoints:

> curl http://52.88.8.107:9650/ext/health
>
> {"checks":{"C":{"message":{"consensus":{"longestRunningBlock":"0s","outstandingBlocks":0},"vm":null},"timestamp":"2023-03-14T15:55:08.256378577Z","duration":14020},"F1c3GayrV5E7NQdxUCbVfnRPsMSxGbTuHvJLS4R48M3Dcazs6":{"message":{"consensus":{"longestRunningBlock":"0s","outstandingBlocks":0},"vm":{"database":{"v1.4.5":null},"health":200}},"timestamp":"2023-03-14T15:55:08.257445968Z","duration":1061711},"P":{"message":{"consensus":{"longestRunningBlock":"0s","outstandingBlocks":0},"vm":{"2km1PAKNK4vSfpyjgK2iFFoBFD3mXHBAWfDBVsPuzKtNYdPRTY-percentConnected":1,"primary-percentConnected":1}},"timestamp":"2023-03-14T15:55:08.256454548Z","duration":74871},"X":{"message":{"consensus":{"outstandingVertices":0,"snowstorm":{"outstandingTransactions":0}},"vm":null},"timestamp":"2023-03-14T15:55:08.256459848Z","duration":15850},"bootstrapped":{"message":[],"timestamp":"2023-03-14T15:55:08.256438778Z","duration":6020},"database":{"timestamp":"2023-03-14T15:55:08.256464299Z","duration":1420},"diskspace":{"message":{"availableDiskBytes":299770843136},"timestamp":"2023-03-14T15:55:08.256466309Z","duration":4070},"network":{"message":{"connectedPeers":5,"sendFailRate":0,"timeSinceLastMsgReceived":"256.459548ms","timeSinceLastMsgSent":"256.459548ms"},"timestamp":"2023-03-14T15:55:08.256461929Z","duration":5551},"router":{"message":{"longestRunningRequest":"0s","outstandingRequests":0},"timestamp":"2023-03-14T15:55:08.256363507Z","duration":17231}},"healthy":true}

> curl http://44.224.148.127:9650/ext/health
>
> {"checks":{"C":{"message":{"consensus":{"longestRunningBlock":"0s","outstandingBlocks":0},"vm":null},"timestamp":"2023-03-14T15:55:08.203325635Z","duration":6570},"F1c3GayrV5E7NQdxUCbVfnRPsMSxGbTuHvJLS4R48M3Dcazs6":{"message":{"consensus":{"longestRunningBlock":"0s","outstandingBlocks":0},"vm":{"database":{"v1.4.5":null},"health":200}},"timestamp":"2023-03-14T15:55:08.204161784Z","duration":927270},"P":{"message":{"consensus":{"longestRunningBlock":"0s","outstandingBlocks":0},"vm":{"2km1PAKNK4vSfpyjgK2iFFoBFD3mXHBAWfDBVsPuzKtNYdPRTY-percentConnected":1,"primary-percentConnected":1}},"timestamp":"2023-03-14T15:55:08.203318015Z","duration":63060},"X":{"message":{"consensus":{"outstandingVertices":0,"snowstorm":{"outstandingTransactions":0}},"vm":null},"timestamp":"2023-03-14T15:55:08.203233714Z","duration":18360},"bootstrapped":{"message":[],"timestamp":"2023-03-14T15:55:08.203205494Z","duration":3010},"database":{"timestamp":"2023-03-14T15:55:08.203214634Z","duration":1350},"diskspace":{"message":{"availableDiskBytes":299770839040},"timestamp":"2023-03-14T15:55:08.203252895Z","duration":5361},"network":{"message":{"connectedPeers":5,"sendFailRate":0,"timeSinceLastMsgReceived":"203.209294ms","timeSinceLastMsgSent":"203.209294ms"},"timestamp":"2023-03-14T15:55:08.203212714Z","duration":6550},"router":{"message":{"longestRunningRequest":"0s","outstandingRequests":0},"timestamp":"2023-03-14T15:55:08.203338225Z","duration":21220}},"healthy":true}

## Deploy TokenVM in Fuji network with avalanche-ops

Same as above, except you do not need anchor nodes:

```bash
/home/ubuntu/avalanche-ops/target/release/avalancheup-aws default-spec \
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
