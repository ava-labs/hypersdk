# Testing

Let's quickly recap what we've done so far:

- We've built out the necessary components of MorpheusVM
- We've extended our implementation by adding an option which creates a JSON-RPC
  server

In theory, our implementation of MorpheusVM should work exactly like the
canonical version. To put this to the test, we're going to apply the same
workload tests used against the canonical version towards our implementation.

This section will consist of the following:

- Implementing a bash script to run our workload tests
- Implementing the workload tests itself
- Implementing a way to feed our workload tests to the HyperSDK workload test
  framework

## Workload Script

We start by reusing the workload script in MorpheusVM. In the tutorial
directory, create a new file called `tests.integration.sh` and paste the
following code:

```bash
#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -e

if ! [[ "$0" =~ tests.integration.sh ]]; then
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
. \
--vms 3

# output generate coverage html
go tool cover -html=integration.coverage.out -o=integration.coverage.html
```

This workload script will both set up our testing environent while also running
our workload tests against our VM.

## Workload Tests

Start by creating a new file called `workload.go`. Afterwards, we implement the
following:

```golang
package tutorial

import (
 "time"

 "github.com/ava-labs/hypersdk/auth"
 "github.com/ava-labs/hypersdk/codec"
 "github.com/ava-labs/hypersdk/crypto/ed25519"
 "github.com/ava-labs/hypersdk/tests/workload"
  "github.com/ava-labs/hypersdk/api/jsonrpc"
  "github.com/ava-labs/hypersdk/api/indexer"
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
 ed25519AddrStrs      = make([]string, len(ed25519HexKeys))
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
  ed25519AddrStrs[i] = codec.MustAddressBech32(HRP, addr)
 }
}

type workloadFactory struct {
 factories []*auth.ED25519Factory
 addrs     []codec.Address
}
```

Our workload tests revolves around the workflow of creating transactions,
sending them to an instance of MorpheusVM, and checking that the correct state
transitions occurred. The code above sets up the accounts required to sign and
send the transactions. We now implement the rest of our workload tests:

```golang

func NewWorkload(minBlockGap int64) (*genesis.DefaultGenesis, workload.TxWorkloadFactory, error) {
 customAllocs := make([]*genesis.CustomAllocation, 0, len(ed25519AddrStrs))
 for _, prefundedAddrStr := range ed25519AddrStrs {
  customAllocs = append(customAllocs, &genesis.CustomAllocation{
   Address: prefundedAddrStr,
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
 lcli := NewJSONRPCClient(uri)
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
 lcli    *JSONRPCClient
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
 aotherStr := codec.MustAddressBech32(HRP, aother)
 parser, err := g.lcli.Parser(ctx)
 if err != nil {
  return nil, nil, err
 }
 _, tx, _, err := g.cli.GenerateTransaction(
  ctx,
  parser,
  []chain.Action{&Transfer{
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
  lcli := NewJSONRPCClient(uri)
  balance, err := lcli.Balance(ctx, aotherStr)
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
 lcli := NewJSONRPCClient(uri)

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
 lcli                *JSONRPCClient
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
 receiverAddrStr := codec.MustAddressBech32(HRP, receiver.address)
 expectedBalance := g.balance - 1_000_000

 parser, err := g.lcli.Parser(ctx)
 if err != nil {
  return nil, nil, err
 }
 _, tx, _, err := g.cli.GenerateTransaction(
  ctx,
  parser,
  []chain.Action{&Transfer{
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
  lcli := NewJSONRPCClient(uri)
  balance, err := lcli.Balance(ctx, receiverAddrStr)
  require.NoError(err)
  require.Equal(expectedBalance, balance)
  // TODO check tx fee + units (not currently available via API)
 }, nil
}
```

Having now defined our workload tests, we now define the logic for feeding them
to the HyperSDK workload test framework.

## Feeding Our Workload Tests

Start by creating a new file called `integration_test.go`. Afterwards, we
implement the following code:

```golang
// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tutorial

import (
 "encoding/json"
 "testing"

 "github.com/stretchr/testify/require"

 "github.com/ava-labs/hypersdk/auth"
 "github.com/ava-labs/hypersdk/crypto/ed25519"
 "github.com/ava-labs/hypersdk/tests/integration"

 ginkgo "github.com/onsi/ginkgo/v2"
)

func TestIntegration(t *testing.T) {
 ginkgo.RunSpecs(t, "morpheusvm integration test suites")
}

var _ = ginkgo.BeforeSuite(func() {
 require := require.New(ginkgo.GinkgoT())
 genesis, workloadFactory, err := NewWorkload(0 /* minBlockGap: 0ms */)
 require.NoError(err)

 genesisBytes, err := json.Marshal(genesis)
 require.NoError(err)

 randomEd25519Priv, err := ed25519.GeneratePrivateKey()
 require.NoError(err)

 randomEd25519AuthFactory := auth.NewED25519Factory(randomEd25519Priv)

 // Setup imports the integration test coverage
 integration.Setup(
  New,
  genesisBytes,
  ID,
  CreateParser,
  JSONRPCEndpoint,
  workloadFactory,
  randomEd25519AuthFactory,
 )
})
```

Our workload tests are really just integration tests which utilize the `ginkgo`
package. In `integration_test.go`, we are doing the following:

- Creating the components specific to our VM, which include:
  - The VM genesis
  - The VM initialization function

  We also create a ED25519 private key and a ED25519 factory as well
- We pass the values above to the HyperSDK workload test framework, which will
  run the workload tests for us.

With the above implemented, it's finally time to test our VM! To run the
workload tests, execute the following command:

```bash
./tests.integration.sh
```

If all goes well, you should see the following message in your command line:

```bash
Ran 11 of 11 Specs in 1.053 seconds
SUCCESS! -- 11 Passed | 0 Failed | 0 Pending | 0 Skipped
PASS
coverage: 61.9% of statements in github.com/ava-labs/hypersdk/...
composite coverage: 60.3% of statements

Ginkgo ran 1 suite in 13.269709292s
Test Suite Passed
```

## Conclusion

In this section, we utilized the MorpheusVM workload tests to test our own
implementation of MorpheusVM.

Overall, we've accomplished the following in this tutorial:

- Implemented a neccesities-only version of MorpheusVM
- Extended MorpheusVM by adding options
- Testing our implementation by utilizing workload tests
