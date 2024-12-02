# Deploying Our VM

In the previous sections, we built our own implementation of MorpheusVM. In this
section, we'll deploy our VM via the Avalanche-CLI and interact with it using
the HyperSDK-CLI. This section consists of the following subsections:

- Setting up Avalanche-CLI
- Defining our VM Genesis
- Creating our VM binary
- Installing our CLI
- Setting up our network
- Setting up the CLI
- Interacting with MorpheusVM via the CLI

Let's get started!

## Avalanche-CLI Installation

To head over, follow the installation instructions in the Avalanche-CLI [README](https://github.com/ava-labs/avalanche-cli?tab=readme-ov-file#installation).

To verify that your version is at least `2.0.0`, run `avalanche --version`,
which should give you the following output:

```bash
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

With our VM binary generator defined, we can now generate our binary with the
following:

```bash
go build -o ./build/tutorial ./cmd/tutorialvm
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

## Network Setup

We first create our blockchain:

```bash
avalanche blockchain create tutorial
```

These are the options you should select:

- Create a Custom-VM
- When asked to enter a genesis, input `./genesis.json`
- Select that you already have a VM binary
- When asked to enter a binary, input `./build/tutorial`

If all goes well, you should see the following:

```bash
✓ Successfully created blockchain configuration
```

With our blockchain created, we'll now deploy it! To do this, run the following:

```bash
avalanche blockchain deploy tutorial --local
```

If all goes well, you'll see a very long message with the following:

```bash
Blockchain ready to use

+---------------------------------------------------------------------------------------------------------------+
|                                                    TUTORIAL                                                   |
+---------------+-----------------------------------------------------------------------------------------------+
| Name          | tutorial                                                                                      |
+---------------+-----------------------------------------------------------------------------------------------+
| VM ID         | tHnUBreJu9Vriu2SNqh7zbCVWnQTcopkVcKcFdvezWjG65NoK                                             |
+---------------+-----------------------------------------------------------------------------------------------+
| VM Version    |                                                                                               |
+---------------+--------------------------+--------------------------------------------------------------------+
| Local Network | SubnetID                 | 2DkbYSegR1iRSFpxV63Ef1QFQXahuux7aDPh6BbWNpMNHgHJiy                 |
|               +--------------------------+--------------------------------------------------------------------+
|               | Owners (Threhold=1)      | P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p                    |
|               +--------------------------+--------------------------------------------------------------------+
|               | BlockchainID (CB58)      | BY3yRXon1VLZwhmWkS5V2BRBScWRaBqaVvhNqv5QMihyZmWDq                  |
|               +--------------------------+--------------------------------------------------------------------+
|               | BlockchainID (HEX)       | 0x17ebfd99e53b621ae5255d291de71a76885f64039fec13d8680aace76a222420 |
+---------------+--------------------------+--------------------------------------------------------------------+

+---------------------------+
|           TOKEN           |
+--------------+------------+
| Token Name   | Test Token |
+--------------+------------+
| Token Symbol | TEST       |
+--------------+------------+

+------------------------------------------------------------------------------------------------+
|                                        TUTORIAL RPC URLS                                       |
+-----------+------------------------------------------------------------------------------------+
| Localhost | http://127.0.0.1:9650/ext/bc/tutorial/rpc                                          |
|           +------------------------------------------------------------------------------------+
|           | http://127.0.0.1:9650/ext/bc/BY3yRXon1VLZwhmWkS5V2BRBScWRaBqaVvhNqv5QMihyZmWDq/rpc |
+-----------+------------------------------------------------------------------------------------+

+--------------------------------------------------------------------------+
|                                   NODES                                  |
+-------+------------------------------------------+-----------------------+
| NAME  | NODE ID                                  | LOCALHOST ENDPOINT    |
+-------+------------------------------------------+-----------------------+
| node1 | NodeID-7Xhw2mDxuDS44j42TCB6U5579esbSt3Lg | http://127.0.0.1:9650 |
+-------+------------------------------------------+-----------------------+
| node2 | NodeID-MFrZFVCXPv5iCn6M9K6XduxGTYp891xXZ | http://127.0.0.1:9652 |
+-------+------------------------------------------+-----------------------+
| node3 | NodeID-NFBbbJ4qCmNaCzeW7sxErhvWqvEQMnYcN | http://127.0.0.1:9654 |
+-------+------------------------------------------+-----------------------+
| node4 | NodeID-GWPcbFJZFfZreETSoWjPimr846mXEKCtu | http://127.0.0.1:9656 |
+-------+------------------------------------------+-----------------------+
| node5 | NodeID-P7oB2McjBGgW2NXXWVYjV8JEDFoW9xDE5 | http://127.0.0.1:9658 |
+-------+------------------------------------------+-----------------------+

+-------------------------------------------------------------+
|                      WALLET CONNECTION                      |
+-----------------+-------------------------------------------+
| Network RPC URL | http://127.0.0.1:9650/ext/bc/tutorial/rpc |
+-----------------+-------------------------------------------+
| Network Name    | tutorial                                  |
+-----------------+-------------------------------------------+
| Chain ID        |                                           |
+-----------------+-------------------------------------------+
| Token Symbol    | TEST                                      |
+-----------------+-------------------------------------------+
| Token Name      | Test Token                                |
+-----------------+-------------------------------------------+
```

We're now ready to use the HyperSDK-CLI!

## CLI Setup

We want to store the private key of our (test!) account
and the RPC endpoint. We first store the RPC endpoint:

```bash
./hypersdk-cli endpoint set --endpoint=http://localhost:9650/ext/bc/tutorial/ 

# Output
Endpoint set to: http://localhost:9650/ext/bc/tutorial/
```

Now storing our private key:

```bash
./hypersdk-cli key set --key=0x323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7

# Output
✅ Key added successfully! 
Address: 0x00c4cb545f748a28770042f893784ce85b107389004d6a0e0d6d7518eeae1292d9
```

We're now ready to interact with our implementation of MorpheusVM!

## Interacting with Tutorial

As a sanity test, let's first check that we can interact with our running VM by
running the following:

```bash
./hypersdk-cli ping
```

If successful, you should see the following:

```bash
✅ Ping succeeded
```

Next, let's see the current balance of our account. We'll run the following
command:

```bash
./hypersdk-cli balance
```

This should give us the following result:

```bash
✅ Balance: 10000000000000
```

Since the account we are using is specified as a prefunded account in the
genesis of our VM (via `DefaultGenesis`), our account balance is as expected.
Let's now write to our VM by sending a TX via the CLI with the following action:

- Transfer
  - Recipient: the zero address
  - Value: 12
  - Memo: "Hello World!" (in hex)

With our action specified, let's call the following command:

```bash
./hypersdk-cli tx Transfer --data to=0x000000000000000000000000000000000000000000000000000000000000000000,value=12,memo=0x48656c6c6f20576f726c6421
```

If all goes well, you should see the following:

```bash
✅ Transaction successful (txID: Cg6N7x6Z2apwMc46heJ8mFMFk2H9CEhNxiUsicrNMnDbyC3ZU)
sender_balance: 9999999969888
receiver_balance: 12
```

Congrats! You've just sent a transaction to your implementation of MorpheusVM.
To double check that your transaction did indeed go through, we can again query
the balance of our account:

```bash
./hypersdk-cli balance
✅ Balance: 9999999969888
```

However, the CLI is not just limited to just the `Transfer` action. To see what
actions you can call, you can use the following:

```bash
./hypersdk-cli actions

---
Transfer

Inputs:
  to: Address
  value: uint64
  memo: []uint8

Outputs:
  sender_balance: uint64
  receiver_balance: uint64
```

Now that we have a good idea of what we can do via the HyperSDK-CLI, it's time
to shut down our VM. To do this, we can run the following:

```bash
avalanche network stop
```

If all went well, you should see the following:

```bash
Network stopped successfully.
Server shutdown gracefully
```

## Conclusion

In this section of the MorpheusVM tutorial, we were able to interact with our
implementation of MorpheusVM by using the HyperSDK-CLI.
