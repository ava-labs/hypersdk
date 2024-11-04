# Interlude

In the previous sections, we built our own implementation of MorpheusVM and
tested it to see that it worked as expected. In the upcoming sections, we will
be completing our VM journey by spinning up `TutorialVM` on a local network and
interacting with it via the HyperSDK-CLI.

However, before we get to the exciting part of this tutorial series, we need to
have some things set up before we can spin up `TutorialVM` on a local network.
In this interlude, we will focus on the following:

- Setting up our run/stop scripts
- Adding end-to-end tests
- Setting up our VM binary generator
- Installing our CLI

Let's get started!

## Script Setup

The script that we'll be using will allow us to start and stop a local network
running `TutorialVM`. To get started, in `examples/tutorial`, run the following commands:

```bash
cp ../morpheusvm/scripts/run.sh ./scripts/run.sh
cp ../morpheusvm/scripts/stop.sh ./scripts/stop.sh

chmod +x ./scripts/run.sh
chmod +x ./scripts/stop.sh
```

The commands above created a new folder named `scripts` and copied the run/stop
scripts from MorpheusVM into our scripts folder, along with giving them
execute permissions.

Before moving for3ward, in lines 68-70, make sure to change it from this:

```bash
go build \
-o "${HYPERSDK_DIR}"/avalanchego-"${VERSION}"/plugins/qCNyZHrs3rZX458wPJXPJJypPf6w423A84jnfbdP2TPEmEE9u \
./cmd/morpheusvm
```

to this:

```bash
go build \
-o "${HYPERSDK_DIR}"/avalanchego-"${VERSION}"/plugins/qCNyZHrs3rZX458wPJXPJJypPf6w423A84jnfbdP2TPEmEE9u \
./cmd/tutorialvm
```

## Adding End-to-End Tests

The scripts above, while extremely useful, work only if we define end-to-end
(e2e) tests for our VM. Since we don't focus on e2e tests in this tutorial, we
can just copy-and-paste a predefined implementation in. To do this, first run
the following:

```bash
mkdir tests/e2e
touch tests/e2e/e2e_test.go
```

Then, in e2e_test.go, write the following:

```go
// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package e2e_test

import (
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/tests/fixture/e2e"
	"github.com/stretchr/testify/require"

	_ "github.com/ava-labs/hypersdk/examples/tutorial/tests" // include the tests that are shared between the integration and e2e

	"github.com/ava-labs/hypersdk/abi"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/examples/tutorial/consts"
	"github.com/ava-labs/hypersdk/examples/tutorial/tests/workload"
	"github.com/ava-labs/hypersdk/examples/tutorial/vm"
	"github.com/ava-labs/hypersdk/tests/fixture"

	he2e "github.com/ava-labs/hypersdk/tests/e2e"
	ginkgo "github.com/onsi/ginkgo/v2"
)

const owner = "tutorial-e2e-tests"

var flagVars *e2e.FlagVars

func TestE2e(t *testing.T) {
	ginkgo.RunSpecs(t, "tutorial e2e test suites")
}

func init() {
	flagVars = e2e.RegisterFlags()
}

// Construct tmpnet network with a single tutorial Subnet
var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {
	require := require.New(ginkgo.GinkgoT())

	testingNetworkConfig, err := workload.NewTestNetworkConfig(100 * time.Millisecond)
	require.NoError(err)

	expectedABI, err := abi.NewABI(vm.ActionParser.GetRegisteredTypes(), vm.OutputParser.GetRegisteredTypes())
	require.NoError(err)

	firstKey := testingNetworkConfig.Keys()[0]
	generator := workload.NewTxGenerator(firstKey)
	spamKey := &auth.PrivateKey{
		Address: auth.NewED25519Address(firstKey.PublicKey()),
		Bytes:   firstKey[:],
	}
	tc := e2e.NewTestContext()
	he2e.SetWorkload(testingNetworkConfig, generator, expectedABI, nil, spamKey)

	return fixture.NewTestEnvironment(tc, flagVars, owner, testingNetworkConfig, consts.ID).Marshal()
}, func(envBytes []byte) {
	// Run in every ginkgo process

	// Initialize the local test environment from the global state
	e2e.InitSharedTestEnvironment(ginkgo.GinkgoT(), envBytes)
})

```

## Adding a VM Binary Generator

Although the run/stop scripts we just created will take care of spinning up and
stopping our network, it still requires us to define a way for our VM binary to
be generated. 

To start, let's run the following commands:

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

## CLI Installation

To start, we'll want to compile the HyperSDK-CLI. If you're in
`examples/tutorial`, you can run the following:

```bash
go install github.com/ava-labs/hypersdk/cmd/hypersdk-cli@4510f51720d2e0fdecfd7fa08350e7c3eab3cf53
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
