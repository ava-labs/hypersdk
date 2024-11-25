# Interlude

In the previous sections, we built and tested our own implementation of
MorpheusVM. In the upcoming sections, we'll be spinning up `TutorialVM` on a
local network and interacting with it via the HyperSDK-CLI.

In this section, we'll:

- Setting up the Avalanche-CLI
- Defining our Genesis
- Setting up our VM binary generator
- Installing our CLI

Let's get started!

### Avalanche-CLI Installation

Start by running the following in your command line:

```bash
curl -sSfL https://raw.githubusercontent.com/ava-labs/avalanche-cli/main/scripts/install.sh | sh -s
```

This will install the `Avalanche-CLI` binary into `~/bin`. Next, you'll need to
add the following to your `.bashrc` file:

```bash
export PATH=~/bin:$PATH
```

After restarting your terminal, running `avalanche` should give you the
following:

```bash
Usage:
  avalanche [command]

Available Commands:
  blockchain            Create and deploy blockchains
  config                Modify configuration for Avalanche-CLI
  contract              Manage smart contracts
  help                  Help about any command
  ictt                  Manage Interchain Token Transferrers (shorthand for `interchain TokenTransferrer`)
  interchain            Set and manage interoperability between blockchains
  key                   Create and manage testnet signing keys
  network               Manage locally deployed subnets
  node                  Set up fuji and mainnet validator on cloud service
  primary               Interact with the Primary Network
  subnet                Create and deploy blockchains (deprecation notice: use 'avalanche blockchain')
  teleporter            Interact with teleporter-enabled subnets
  transaction           Sign and execute specific transactions
  update                Check for latest updates of Avalanche-CLI

Flags:
      --config string       config file (default is $HOME/.avalanche-cli/config.json)
  -h, --help                help for avalanche
      --log-level string    log level for the application (default "ERROR")
      --skip-update-check   skip check for new versions
  -v, --version             version for avalanche

Use "avalanche [command] --help" for more information about a command.
```

To verify that your version is at least `2.0.0`, run the following:

```bash
AVL-0W5L7Y:avalanche-cli rodrigo.villar$ avalanche --version
avalanche version 2.0.0
```

## Adding our Genesis

In `/tutorial`, create a file named `genesis.json` and paste the following:

```json
{
    "stateBranchFactor": 16,
    "customAllocation": [
        {
            "address": "0x00c4cb545f748a28770042f893784ce85b107389004d6a0e0d6d7518eeae1292d914969017",
            "balance": 10000000000000
        },
        {
            "address": "0x003020338128fc7babb4e5850aace96e589f55b33bda90d62c44651de110ea5b8c0b5ee37f",
            "balance": 10000000000000
        }
    ],
    "initialRules": {
        "minBlockGap": 100,
        "minEmptyBlockGap": 750,
        "minUnitPrice": {
            "bandwidth": 100,
            "compute": 100,
            "storageRead": 100,
            "storageAllocate": 100,
            "storageWrite": 100
        },
        "unitPriceChangeDenominator": {
            "bandwidth": 48,
            "compute": 48,
            "storageRead": 48,
            "storageAllocate": 48,
            "storageWrite": 48
        },
        "windowTargetUnits": {
            "bandwidth": 18446744073709551615,
            "compute": 18446744073709551615,
            "storageRead": 18446744073709551615,
            "storageAllocate": 18446744073709551615,
            "storageWrite": 18446744073709551615
        },
        "maxBlockUnits": {
            "bandwidth": 1800000,
            "compute": 18446744073709551615,
            "storageRead": 18446744073709551615,
            "storageAllocate": 18446744073709551615,
            "storageWrite": 18446744073709551615
        },
        "validityWindow": 60000,
        "maxActionsPerTx": 16,
        "maxOutputsPerAction": 1,
        "baseUnits": 1,
        "storageKeyReadUnits": 5,
        "storageValueReadUnits": 2,
        "storageKeyAllocateUnits": 20,
        "storageValueAllocateUnits": 5,
        "storageKeyWriteUnits": 10,
        "storageValueWriteUnits": 3,
        "sponsorStateKeysMaxChunks": [
            1
        ]
    }
}

```

## Adding a VM Binary Generator

We'll now write a simple CLI that allows us to generate the binary for
`TutorialVM`. To start, let's run the following commands:

```bash
mkdir -p cmd/tutorialvm
mkdir cmd/tutorialvm/version
touch cmd/tutorialvm/version/version.go
touch cmd/tutorialvm/main.go
```

Let's first focus on `main.go`. Here, let's implement the following:

```go
// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/ulimit"
	"github.com/ava-labs/avalanchego/vms/rpcchainvm"
	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/examples/tutorial/cmd/tutorialvm/version"
	"github.com/ava-labs/hypersdk/examples/tutorial/vm"
)

var rootCmd = &cobra.Command{
	Use:        "morpheusvm",
	Short:      "BaseVM agent",
	SuggestFor: []string{"morpheusvm"},
	RunE:       runFunc,
}

func init() {
	cobra.EnablePrefixMatching = true
}

func init() {
	rootCmd.AddCommand(
		version.NewCommand(),
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "morpheusvm failed %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func runFunc(*cobra.Command, []string) error {
	if err := ulimit.Set(ulimit.DefaultFDLimit, logging.NoLog{}); err != nil {
		return fmt.Errorf("%w: failed to set fd limit correctly", err)
	}

	vm, err := vm.New()
	if err != nil {
		return err
	}
	return rpcchainvm.Serve(context.TODO(), vm)
}
```

Next, in `version.go`, let's implement the following:

```go
// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package version

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/examples/tutorial/consts"
)

func init() {
	cobra.EnablePrefixMatching = true
}

// NewCommand implements "morpheusvm version" command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Prints out the verson",
		RunE:  versionFunc,
	}
	return cmd
}

func versionFunc(*cobra.Command, []string) error {
	fmt.Printf("%s@%s (%s)\n", consts.Name, consts.Version, consts.ID)
	return nil
}
```

## HyperSDK-CLI Installation

To start, install the HyperSDK-CLI by running the following:

```bash
go install github.com/ava-labs/hypersdk/cmd/hypersdk-cli@b2ad4d38aec5b2958a02b209b58eafc6891c51cd
```

To confirm that your build of the HyperSDK-CLI was successful, run the following
command:

```bash
hypersdk-cli
```

You should see the following:

```bash
A CLI application for performing read and write actions on HyperSDK-based chains.

Usage:
  hypersdk-cli [command]

Available Commands:
  actions     Print the list of actions available in the ABI
  address     Print current key address
  balance     Get the balance of an address
  completion  Generate the autocompletion script for the specified shell
  endpoint    Manage endpoint
  help        Help about any command
  key         Manage keys
  ping        Ping the endpoint
  read        Read data from the chain
  tx          Execute a transaction on the chain

Flags:
      --endpoint string   Override the default endpoint
  -h, --help              help for hypersdk-cli
      --key string        Private ED25519 key as hex string
  -o, --output string     Output format (text or json) (default "text")

Use "hypersdk-cli [command] --help" for more information about a command.
```

With all the above set up, we're now ready to use the CLI!
