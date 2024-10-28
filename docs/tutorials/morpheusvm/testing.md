# Testing

Let's quickly recap what we've done so far:

- We've built a base implementation of MorpheusVM
- We've extended our implementation by adding a JSON-RPC server option

With the above, our code should work exactly like the version of MorpheusVM
found in `examples/`. To verify this though, we're going to apply the same
workload tests used in MorpheusVM against our VM. 

This section will consist of the following:

- Implementing a bash script to run our workload tests
- Implementing the workload tests itself
- Implementing a way to feed our workload tests to the HyperSDK workload test
  framework

## Workload Scripts

We start by reusing the workload script from MorpheusVM. In `tutorial/`, create
a new directory named `scripts`. Within this scripts directory, create a file
called `tests.integration.sh` and paste the following:

```bash
#!/usr/bin/env bash

set -e

if ! [[ "$0" =~ scripts/tests.integration.sh ]]; then
  echo "must be run from tutorial root"
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

This workload script will both set up our testing environment and execute the
workload tests. To make sure that our script will run at the end of this
section, run the following command:

```bash
chmod +x ./scripts/tests.integration.sh
```

## Implementing Our Workload Tests

Start by creating a subdirectory in `tutorial/` named `tests`. Within `tests/`,
create a directory called `workload`. Within `workload/`, create a file called `workload.go`. This file is where we will
define the workload tests that will be used against our VM. 

## Workload Initialization

We start by defining the values necesssary for the workload tests along with an
`init()` function:

```golang
package workload

import (
	"context"
	"math"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/api/indexer"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/bls"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/crypto/secp256r1"
	"github.com/ava-labs/hypersdk/examples/tutorial/actions"
	"github.com/ava-labs/hypersdk/examples/tutorial/consts"
	"github.com/ava-labs/hypersdk/examples/tutorial/vm"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/tests/workload"
)

const (
	initialBalance  uint64 = 10_000_000_000_000
	txCheckInterval        = 100 * time.Millisecond
)

var (
	_              workload.TxWorkloadFactory  = (*workloadFactory)(nil)
	_              workload.TxWorkloadIterator = (*simpleTxWorkload)(nil)
	ed25519HexKeys                             = []string{
		"323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7", //nolint:lll
		"8a7be2e0c9a2d09ac2861c34326d6fe5a461d920ba9c2b345ae28e603d517df148735063f8d5d8ba79ea4668358943e5c80bc09e9b2b9a15b5b15db6c1862e88", //nolint:lll
	}
	ed25519PrivKeys      = make([]ed25519.PrivateKey, len(ed25519HexKeys))
	ed25519Addrs         = make([]codec.Address, len(ed25519HexKeys))
	ed25519AuthFactories = make([]*auth.ED25519Factory, len(ed25519HexKeys))
)

func init() {
	for i, keyHex := range ed25519HexKeys {
		privBytes, err := codec.LoadHex(keyHex, ed25519.PrivateKeyLen)
		if err != nil {
			panic(err)
		}
		priv := ed25519.PrivateKey(privBytes)
		ed25519PrivKeys[i] = priv
		ed25519AuthFactories[i] = auth.NewED25519Factory(priv)
		addr := auth.NewED25519Address(priv.PublicKey())
		ed25519Addrs[i] = addr
	}
}
```

Our workload tests revolve around the following workflow:

- Use transaction workloads to generate an arbitrary TX
  - This TX contains just a `Transfer` action
- Send generated TX to a running instance of our VM
- Assert that the TX execute and applied the correct state changes

The code snippet above provides the foundation to define `workloadFactory` and
`simpleTxWorkload`. We now implement these structs along with their methods:

```golang
type workloadFactory struct {
	factories []*auth.ED25519Factory
	addrs     []codec.Address
}

func New(minBlockGap int64) (*genesis.DefaultGenesis, workload.TxWorkloadFactory, error) {
	customAllocs := make([]*genesis.CustomAllocation, 0, len(ed25519Addrs))
	for _, prefundedAddr := range ed25519Addrs {
		customAllocs = append(customAllocs, &genesis.CustomAllocation{
			Address: prefundedAddr,
			Balance: initialBalance,
		})
	}

	genesis := genesis.NewDefaultGenesis(customAllocs)
	// Set WindowTargetUnits to MaxUint64 for all dimensions to iterate full mempool during block building.
	genesis.Rules.WindowTargetUnits = fees.Dimensions{math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64}
	// Set all limits to MaxUint64 to avoid limiting block size for all dimensions except bandwidth. Must limit bandwidth to avoid building
	// a block that exceeds the maximum size allowed by AvalancheGo.
	genesis.Rules.MaxBlockUnits = fees.Dimensions{1800000, math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64}
	genesis.Rules.MinBlockGap = minBlockGap

	return genesis, &workloadFactory{
		factories: ed25519AuthFactories,
		addrs:     ed25519Addrs,
	}, nil
}

func (f *workloadFactory) NewSizedTxWorkload(uri string, size int) (workload.TxWorkloadIterator, error) {
	cli := jsonrpc.NewJSONRPCClient(uri)
	lcli := vm.NewJSONRPCClient(uri)
	return &simpleTxWorkload{
		factory: f.factories[0],
		cli:     cli,
		lcli:    lcli,
		size:    size,
	}, nil
}

type simpleTxWorkload struct {
	factory *auth.ED25519Factory
	cli     *jsonrpc.JSONRPCClient
	lcli    *vm.JSONRPCClient
	count   int
	size    int
}

func (g *simpleTxWorkload) Next() bool {
	return g.count < g.size
}

func (g *simpleTxWorkload) GenerateTxWithAssertion(ctx context.Context) (*chain.Transaction, workload.TxAssertion, error) {
	g.count++
	other, err := ed25519.GeneratePrivateKey()
	if err != nil {
		return nil, nil, err
	}

	aother := auth.NewED25519Address(other.PublicKey())
	parser, err := g.lcli.Parser(ctx)
	if err != nil {
		return nil, nil, err
	}
	_, tx, _, err := g.cli.GenerateTransaction(
		ctx,
		parser,
		[]chain.Action{&actions.Transfer{
			To:    aother,
			Value: 1,
		}},
		g.factory,
	)
	if err != nil {
		return nil, nil, err
	}

	return tx, func(ctx context.Context, require *require.Assertions, uri string) {
		indexerCli := indexer.NewClient(uri)
		success, _, err := indexerCli.WaitForTransaction(ctx, txCheckInterval, tx.ID())
		require.NoError(err)
		require.True(success)
		lcli := vm.NewJSONRPCClient(uri)
		balance, err := lcli.Balance(ctx, aother)
		require.NoError(err)
		require.Equal(uint64(1), balance)
	}, nil
}

func (f *workloadFactory) NewWorkloads(uri string) ([]workload.TxWorkloadIterator, error) {
	blsPriv, err := bls.GeneratePrivateKey()
	if err != nil {
		return nil, err
	}
	blsPub := bls.PublicFromPrivateKey(blsPriv)
	blsAddr := auth.NewBLSAddress(blsPub)
	blsFactory := auth.NewBLSFactory(blsPriv)

	secpPriv, err := secp256r1.GeneratePrivateKey()
	if err != nil {
		return nil, err
	}
	secpPub := secpPriv.PublicKey()
	secpAddr := auth.NewSECP256R1Address(secpPub)
	secpFactory := auth.NewSECP256R1Factory(secpPriv)

	cli := jsonrpc.NewJSONRPCClient(uri)
	networkID, _, blockchainID, err := cli.Network(context.Background())
	if err != nil {
		return nil, err
	}
	lcli := vm.NewJSONRPCClient(uri)

	generator := &mixedAuthWorkload{
		addressAndFactories: []addressAndFactory{
			{address: f.addrs[1], authFactory: f.factories[1]},
			{address: blsAddr, authFactory: blsFactory},
			{address: secpAddr, authFactory: secpFactory},
		},
		balance:   initialBalance,
		cli:       cli,
		lcli:      lcli,
		networkID: networkID,
		chainID:   blockchainID,
	}

	return []workload.TxWorkloadIterator{generator}, nil
}

type addressAndFactory struct {
	address     codec.Address
	authFactory chain.AuthFactory
}

type mixedAuthWorkload struct {
	addressAndFactories []addressAndFactory
	balance             uint64
	cli                 *jsonrpc.JSONRPCClient
	lcli                *vm.JSONRPCClient
	networkID           uint32
	chainID             ids.ID
	count               int
}

func (g *mixedAuthWorkload) Next() bool {
	return g.count < len(g.addressAndFactories)-1
}

func (g *mixedAuthWorkload) GenerateTxWithAssertion(ctx context.Context) (*chain.Transaction, workload.TxAssertion, error) {
	defer func() { g.count++ }()

	sender := g.addressAndFactories[g.count]
	receiver := g.addressAndFactories[g.count+1]
	expectedBalance := g.balance - 1_000_000

	parser, err := g.lcli.Parser(ctx)
	if err != nil {
		return nil, nil, err
	}
	_, tx, _, err := g.cli.GenerateTransaction(
		ctx,
		parser,
		[]chain.Action{&actions.Transfer{
			To:    receiver.address,
			Value: expectedBalance,
		}},
		sender.authFactory,
	)
	if err != nil {
		return nil, nil, err
	}
	g.balance = expectedBalance

	return tx, func(ctx context.Context, require *require.Assertions, uri string) {
		indexerCli := indexer.NewClient(uri)
		success, _, err := indexerCli.WaitForTransaction(ctx, txCheckInterval, tx.ID())
		require.NoError(err)
		require.True(success)
		lcli := vm.NewJSONRPCClient(uri)
		balance, err := lcli.Balance(ctx, receiver.address)
		require.NoError(err)
		require.Equal(expectedBalance, balance)
		// TODO check tx fee + units (not currently available via API)
	}, nil
}
```

## Using Our Workload Tests

With our workload tests implemented, we now define a way for which our workload
tests can be utilized. To start, create a new folder named `integration` in
`tests/`. Inside `integration/`, create a new file `integration_test.go`. Here
copy-paste the following:

```golang
package integration_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/tutorial/vm"
	"github.com/ava-labs/hypersdk/tests/integration"

	lconsts "github.com/ava-labs/hypersdk/examples/tutorial/consts"
	tutorialWorkload "github.com/ava-labs/hypersdk/examples/tutorial/tests/workload"
	ginkgo "github.com/onsi/ginkgo/v2"
)

func TestIntegration(t *testing.T) {
	ginkgo.RunSpecs(t, "tutorial integration test suites")
}

var _ = ginkgo.BeforeSuite(func() {
	require := require.New(ginkgo.GinkgoT())
	genesis, workloadFactory, err := tutorialWorkload.New(0 /* minBlockGap: 0ms */)
	require.NoError(err)

	genesisBytes, err := json.Marshal(genesis)
	require.NoError(err)

	randomEd25519Priv, err := ed25519.GeneratePrivateKey()
	require.NoError(err)

	randomEd25519AuthFactory := auth.NewED25519Factory(randomEd25519Priv)

	// Setup imports the integration test coverage
	integration.Setup(
		vm.New,
		genesisBytes,
		lconsts.ID,
		vm.CreateParser,
		workloadFactory,
		randomEd25519AuthFactory,
	)
})
```

In `integration_test.go`, we are feeding our workload tests along with various
other values to the HyperSDK integration test library. Implementing an entire
integration test framework is time-intensive. By using the HyperSDK integration
test framework, we can defer most tasks to it and solely focus on defining the
workload tests.

## Testing Our VM

Putting everything together, its now time to test our work! To do this, run the
following command:

```bash
./scripts/tests.integration.sh
```

If all goes well, you should see the following message in your command line:

```bash
Ran 12 of 12 Specs in 1.083 seconds
SUCCESS! -- 12 Passed | 0 Failed | 0 Pending | 0 Skipped
PASS
coverage: 61.8% of statements in github.com/ava-labs/hypersdk/...
composite coverage: 60.1% of statements

Ginkgo ran 1 suite in 9.091254458s
Test Suite Passed
```

If you see this, then this means that your VM passed the workload tests!

## Conclusion

Assuming the above went well, you've just built a VM which is functionally
equivalent to MorpheusVM. In particular, you started by building a base version
of MorpheusVM which introduced the concepts of actions and storage in the
context of token transfers. In the `options` section, you then extended your VM
by adding an option which allowed your VM to spin-up a JSON-RPC server. Finally,
in this section, we added workload tests to make sure our VM works as expected. 
