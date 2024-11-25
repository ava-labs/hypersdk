# Testing

Let's quickly recap what we've done so far:

- We've built a base implementation of MorpheusVM
- We've extended our implementation by adding a JSON-RPC server option
- We deployed our implementation of MorpheusVM and interacted with it

With the above, our code should work exactly like the version of MorpheusVM
found in `examples/`. To verify this though, we're going to apply the same 
workload/`e2e` tests used in MorpheusVM against our VM.

This section will consist of the following:

- Implementing a bash script to run our workload tests
- Implementing workload tests that generate a large quantity of generic transactions
- Implementing workload tests that test for a specific transaction
- Registering our workload tests
- Implementing bash scripts to run `e2e` tests
- Registering our `e2e` tests

## Workload Scripts

We start by reusing the workload script from MorpheusVM. This script, when
called, will execute the workload tests we will define. In `tutorial/`, create
a new directory named `scripts`. Within this scripts directory, create a file
called `tests.integration.sh` and paste the following:

```bash
#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -e

if ! [[ "$0" =~ scripts/tests.integration.sh ]]; then
  echo "must be run from morpheusvm root"
  exit 255
fi

# shellcheck source=/scripts/common/utils.sh
source ../../scripts/common/utils.sh
# shellcheck source=/scripts/constants.sh
source ../../scripts/constants.sh

rm_previous_cov_reports
prepare_ginkgo

# run with 3 embedded VMs
ACK_GINKGO_RC=true ginkgo \
run \
-v \
--fail-fast \
-cover \
-covermode=atomic \
-coverpkg=github.com/ava-labs/hypersdk/... \
-coverprofile=integration.coverage.out \
./tests/integration \
--vms 3

# output generate coverage html
go tool cover -html=integration.coverage.out -o=integration.coverage.html
```

Let's make sure that our script can be executed:

```bash
chmod +x ./scripts/tests.integration.sh
```

## Testing via Transaction Generation

Start by creating a subdirectory in `tutorial/` named `tests`. Within `tests/`,
create a directory called `workload`. Within `workload`, create the following
files:

- `generator.go`
- `genesis.go`

`generator.go` will be responsible for generating transactions that
contain the `Transfer` action while `genesis.go` will be responsible for
providing the network configuration for our tests. 

### Implementing the Generator

In `generator.go`, we start by implementing the following:

```go
package workload

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/api/indexer"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/tutorial/actions"
	"github.com/ava-labs/hypersdk/examples/tutorial/consts"
	"github.com/ava-labs/hypersdk/examples/tutorial/vm"
	"github.com/ava-labs/hypersdk/tests/workload"
)

var _ workload.TxGenerator = (*TxGenerator)(nil)

const txCheckInterval = 100 * time.Millisecond

type TxGenerator struct {
	factory *auth.ED25519Factory
}

func NewTxGenerator(key ed25519.PrivateKey) *TxGenerator {
	return &TxGenerator{
		factory: auth.NewED25519Factory(key),
	}
}
```

Here, we started by creating a `TxGenerator` struct which will be responsible
for generating transactions. Next, we'll want to implement a method to our
`TxGenerator` that will allow it to produce a valid transaction with `Transfer`
on the fly. We have:

```go
func (g *TxGenerator) GenerateTx(ctx context.Context, uri string) (*chain.Transaction, workload.TxAssertion, error) {
	// TODO: no need to generate the clients every tx
	cli := jsonrpc.NewJSONRPCClient(uri)
	lcli := vm.NewJSONRPCClient(uri)

	to, err := ed25519.GeneratePrivateKey()
	if err != nil {
		return nil, nil, err
	}

	toAddress := auth.NewED25519Address(to.PublicKey())
	parser, err := lcli.Parser(ctx)
	if err != nil {
		return nil, nil, err
	}
	_, tx, _, err := cli.GenerateTransaction(
		ctx,
		parser,
		[]chain.Action{&actions.Transfer{
			To:    toAddress,
			Value: 1,
		}},
		g.factory,
	)
	if err != nil {
		return nil, nil, err
	}

	return tx, func(ctx context.Context, require *require.Assertions, uri string) {
		confirmTx(ctx, require, uri, tx.GetID(), toAddress, 1)
	}, nil
}
```

In addition to generating a valid transaction, this method returns an anonymous
function containing `confirmTX`. `confirmTX` sends the generated TX to the VM,
makes sure that it was accepted, and checks that the TX outputs are as expected.

```golang
func confirmTx(ctx context.Context, require *require.Assertions, uri string, txID ids.ID, receiverAddr codec.Address, receiverExpectedBalance uint64) {
	indexerCli := indexer.NewClient(uri)
	success, _, err := indexerCli.WaitForTransaction(ctx, txCheckInterval, txID)
	require.NoError(err)
	require.True(success)
	lcli := vm.NewJSONRPCClient(uri)
	balance, err := lcli.Balance(ctx, receiverAddr)
	require.NoError(err)
	require.Equal(receiverExpectedBalance, balance)
	txRes, _, err := indexerCli.GetTx(ctx, txID)
	require.NoError(err)
	// TODO: perform exact expected fee, units check, and output check
	require.NotZero(txRes.Fee)
	require.Len(txRes.Outputs, 1)
	transferOutputBytes := []byte(txRes.Outputs[0])
	require.Equal(consts.TransferID, transferOutputBytes[0])
	reader := codec.NewReader(transferOutputBytes, len(transferOutputBytes))
	transferOutputTyped, err := vm.OutputParser.Unmarshal(reader)
	require.NoError(err)
	transferOutput, ok := transferOutputTyped.(*actions.TransferResult)
	require.True(ok)
	require.Equal(receiverExpectedBalance, transferOutput.ReceiverBalance)
}
```

In the above, some of the checks that `confirmTx` does are:
- Checking that the TX was successful
- Checking that the balance of the receiver is as expected
- Checking that the balance of the sender is as expected
- Checking that the output of the TX is as expected

With our generator complete, we can now move onto implementing the network
configuration.

### Implementing the Network Configuration

In `genesis.go`, we first start by implementing a function which returns the
genesis of our VM:

```go
// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package workload

import (
	"encoding/json"
	"math"
	"time"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/tutorial/consts"
	"github.com/ava-labs/hypersdk/examples/tutorial/vm"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/tests/workload"
)

const (
	// default initial balance for each address
	InitialBalance uint64 = 10_000_000_000_000
)

var _ workload.TestNetworkConfiguration = &NetworkConfiguration{}

// hardcoded initial set of ed25519 keys. Each will be initialized with InitialBalance
var ed25519HexKeys = []string{
	"323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7", //nolint:lll
	"8a7be2e0c9a2d09ac2861c34326d6fe5a461d920ba9c2b345ae28e603d517df148735063f8d5d8ba79ea4668358943e5c80bc09e9b2b9a15b5b15db6c1862e88", //nolint:lll
}

func newGenesis(keys []ed25519.PrivateKey, minBlockGap time.Duration) *genesis.DefaultGenesis {
	// allocate the initial balance to the addresses
	customAllocs := make([]*genesis.CustomAllocation, 0, len(keys))
	for _, key := range keys {
		customAllocs = append(customAllocs, &genesis.CustomAllocation{
			Address: auth.NewED25519Address(key.PublicKey()),
			Balance: InitialBalance,
		})
	}

	genesis := genesis.NewDefaultGenesis(customAllocs)

	// Set WindowTargetUnits to MaxUint64 for all dimensions to iterate full mempool during block building.
	genesis.Rules.WindowTargetUnits = fees.Dimensions{math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64}

	// Set all limits to MaxUint64 to avoid limiting block size for all dimensions except bandwidth. Must limit bandwidth to avoid building
	// a block that exceeds the maximum size allowed by AvalancheGo.
	genesis.Rules.MaxBlockUnits = fees.Dimensions{1800000, math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64}
	genesis.Rules.MinBlockGap = minBlockGap.Milliseconds()

	genesis.Rules.NetworkID = uint32(1)
	genesis.Rules.ChainID = ids.GenerateTestID()

	return genesis
}
```

In addition to defining chain-specific values, `newGenesis()` also defines two
accounts whose balances will be allocated once the VM is spun up. Next, using 
the values in `ed25519HexKeys`, we'll implement a function that returns our 
private test keys:

```go
func newDefaultKeys() []ed25519.PrivateKey {
	testKeys := make([]ed25519.PrivateKey, len(ed25519HexKeys))
	for i, keyHex := range ed25519HexKeys {
		bytes, err := codec.LoadHex(keyHex, ed25519.PrivateKeyLen)
		if err != nil {
			panic(err)
		}
		testKeys[i] = ed25519.PrivateKey(bytes)
	}

	return testKeys
}
```

Finally, we initialize the workload.DefaultTestNetworkConfiguration required for our VM
tests:

```go

func NewTestNetworkConfig(minBlockGap time.Duration) (*NetworkConfiguration, error) {
	keys := newDefaultKeys()
	genesis := newGenesis(keys, minBlockGap)
	genesisBytes, err := json.Marshal(genesis)
	if err != nil {
		return workload.DefaultTestNetworkConfiguration{}, err
	}
	return workload.NewDefaultTestNetworkConfiguration(
		genesisBytes,
		consts.Name,
		vm.NewParser(genesis),
		keys,
	), nil
}
```

By wrapping our genesis and accounts keys into `NetworkConfiguration`, we can
pass this into the HyperSDKtest library, which will use the fields of the struct
to set up and execute our tests. We now move onto testing against a specific 
transaction.

## Testing via a Specific Transaction

The benefit of this testing style is that it's similar to writing unit tests. 
To start, in the `tests` folder, run the following command:

```bash
touch transfer.go
```

We'll be writing a registry test to test the `Transfer`
action. Within `transfer.go`, we write the following:

```go
// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tests

import (
	"context"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/tutorial/actions"
	"github.com/ava-labs/hypersdk/examples/tutorial/tests/workload"
	"github.com/ava-labs/hypersdk/tests/registry"

	tworkload "github.com/ava-labs/hypersdk/tests/workload"
	ginkgo "github.com/onsi/ginkgo/v2"
)

// TestsRegistry initialized during init to ensure tests are identical during ginkgo
// suite construction and test execution
// ref https://onsi.github.io/ginkgo/#mental-model-how-ginkgo-traverses-the-spec-hierarchy
var TestsRegistry = &registry.Registry{}

var _ = registry.Register(TestsRegistry, "Transfer Transaction", func(t ginkgo.FullGinkgoTInterface, tn tworkload.TestNetwork) {

})
```

In the code above, we have `TestsRegistry`: this is a
registry of all the tests that we want to run against our VM.
Afterwards, we have the following snippet:

```go
registry.Register(TestsRegistry, "Transfer Transaction", func(t ginkgo.FullGinkgoTInterface, tn tworkload.TestNetwork) {

})
```

Here, we are adding a test to `TestRegistry`. However, we're
missing the test itself. In short, here's what we want to do in
our testing logic:

- Setup necessary values
- Create our test TX
- Send our TX
- Require that our TX is sent and that the outputs are as expected

Focusing on the first step, we can write the following inside the anonymous
function:

```go
	require := require.New(t)
	other, err := ed25519.GeneratePrivateKey()
	require.NoError(err)
	toAddress := auth.NewED25519Address(other.PublicKey())

	authFactory := tn.Configuration().AuthFactories()[0]
```

Next, we'll create our test transaction. In short, we'll want to send a value of
`1` to `To`. Therefore, we have:

```go
	tx, err := tn.GenerateTx(context.Background(), []chain.Action{&actions.Transfer{
		To:    toAddress,
		Value: 1,
	}},
		authFactory,
	)
	require.NoError(err)
```

Finally, we'll want to send our TX and do the checks mentioned in the last step.
This step will consist of the following:

- Creating a context with a deadline of 2 seconds
  - If the test takes longer than 2 seconds, it will fail
- Calling `ConfirmTxs` with our TX being passed in

The function `ConfirmTXs` is useful as it checks that our TX was
sent and that, if finalized, our transaction has the expected outputs. We have
the following:

```go
	timeoutCtx, timeoutCtxFnc := context.WithDeadline(context.Background(), time.Now().Add(30*time.Second))
	defer timeoutCtxFnc()

	require.NoError(tn.ConfirmTxs(timeoutCtx, []*chain.Transaction{tx}))
```

## Registering our Tests

Although we've defined the tests themselves, we still need to
register them with the HyperSDK. To start, create a new folder named `integration` in
`tests/`. Inside `integration/`, create a new file `integration_test.go`. Here,
copy-paste the following:

```go
// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package integration_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	_ "github.com/ava-labs/hypersdk/examples/tutorial/tests" // include the tests that are shared between the integration and e2e

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/tutorial/tests/workload"
	"github.com/ava-labs/hypersdk/examples/tutorial/vm"
	"github.com/ava-labs/hypersdk/tests/integration"

	lconsts "github.com/ava-labs/hypersdk/examples/tutorial/consts"
	ginkgo "github.com/onsi/ginkgo/v2"
)

func TestIntegration(t *testing.T) {
	ginkgo.RunSpecs(t, "tutorial integration test suites")
}

var _ = ginkgo.BeforeSuite(func() {
	require := require.New(ginkgo.GinkgoT())

	testingNetworkConfig, err := workload.NewTestNetworkConfig(0)
	require.NoError(err)

	randomEd25519Priv, err := ed25519.GeneratePrivateKey()
	require.NoError(err)

	randomEd25519AuthFactory := auth.NewED25519Factory(randomEd25519Priv)

	generator := workload.NewTxGenerator(testingNetworkConfig.AuthFactories()[0])
	// Setup imports the integration test coverage
	integration.Setup(
		vm.New,
		testingNetworkConfig,
		lconsts.ID,
		generator,
		randomEd25519AuthFactory,
	)
})
```

In `integration_test.go`, we are feeding our tests along with various
other values to the HyperSDK test library. Using this pattern allows
us to defer most tasks to it and solely focus on defining the tests.

## Testing Our VM

Before testing, your tests directory should look as follows:

```
tests
├── integration
│   └── integration_test.go
├── transfer.go
└── workload
    ├── generator.go
    └── genesis.go
```

Putting everything together, it's now time to test our work! To do this, run the
following command:

```bash
./scripts/tests.integration.sh
```

If all goes well, you should see the following message in your command line:

```bash
Ran 12 of 12 Specs in 1.614 seconds
SUCCESS! -- 12 Passed | 0 Failed | 0 Pending | 0 Skipped
PASS
coverage: 61.9% of statements in github.com/ava-labs/hypersdk/...
composite coverage: 60.4% of statements

Ginkgo ran 1 suite in 10.274886041s
Test Suite Passed
```

If you see this, then your VM passed the workload tests!

## Setting Up `e2e` Tests

We'll now focus on adding `e2e` tests to our VM. To get started, in
`examples/tutorial`, run the following commands:

```bash
cp ../morpheusvm/scripts/run.sh ./scripts/run.sh
cp ../morpheusvm/scripts/stop.sh ./scripts/stop.sh

chmod +x ./scripts/run.sh
chmod +x ./scripts/stop.sh
```

The commands above copied the run/stop scripts from MorpheusVM into our scripts folder, along with giving them execute permissions.

Before moving forward, in lines 68-70 of `run.sh`, make sure to change it from this:

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

## Adding `e2e` Tests

A caveat of the scripts above is that we need to define end-to-end (e2e) tests for our VM. To start, run the following:

```bash
mkdir tests/e2e
touch tests/e2e/e2e_test.go
```

Then, in `e2e_test.go`, write the following:

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

	firstAuthFactory := testingNetworkConfig.AuthFactories()[0]
	generator := workload.NewTxGenerator(firstAuthFactory)
	tc := e2e.NewTestContext()
	he2e.SetWorkload(testingNetworkConfig, generator, expectedABI, nil, firstAuthFactory)

	return fixture.NewTestEnvironment(tc, flagVars, owner, testingNetworkConfig, consts.ID).Marshal()
}, func(envBytes []byte) {
	// Run in every ginkgo process

	// Initialize the local test environment from the global state
	e2e.InitSharedTestEnvironment(ginkgo.GinkgoT(), envBytes)
})

```

If the above looks familar to `integration_test.go`, that's because `e2e` tests
follow the same logic as integration tests! The HyperSDK also has a framework
for `e2e` tests, which only requires us to pass in required values like the ABI
and a transaction generator.

## Running `e2e` Tests

To run your `e2e` tests, execute the following:

```bash
MODE=TEST ./scripts/run.sh
```

In your command-line, you should see a sequence of tests being executed.

## Conclusion

Assuming the above went well, you've just verified that your VM is functionally
equivalent to MorpheusVM. 
