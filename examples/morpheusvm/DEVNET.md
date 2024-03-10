# Reproducible Devnet

## Install Avalanche-CLI

```bash
git clone git@github.com:ava-labs/avalanche-cli.git
cd avalanche-cli
./scripts/build.sh
```

## Move Build Avalanche-CLI to bin

```bash
sudo mv ./bin/avalanche /usr/local/bin
```

## Provision Network

```bash
avalanche node devnet wiz vryxTest1 vryxTest1 --num-apis 1,1 --num-validators 2,2 --region us-east-1,us-east-2 --aws --node-type c5.4xlarge --separate-monitoring-instance --default-validator-params --custom-vm-repo-url="https://www.github.com/ava-labs/hypersdk" --custom-vm-branch vryx-poc --custom-vm-build-script="examples/morpheusvm/scripts/build.sh" --custom-subnet=true --subnet-genesis="" --subnet-config="" --node-config="" --chain-config=""
```
