// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package e2e_test

import (
	"encoding/json"
	"testing"

	"github.com/ava-labs/avalanchego/tests/fixture/e2e"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/abi"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/consts"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/vm"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/tests/fixture"

	he2e "github.com/ava-labs/hypersdk/tests/e2e"
	ginkgo "github.com/onsi/ginkgo/v2"
)

const (
	owner                 = "cfmmvm-e2e-tests"
	initialBalance uint64 = 10_000_000_000_000
)

var (
	ed25519HexKeys = []string{
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

var flagVars *e2e.FlagVars

func TestE2e(t *testing.T) {
	ginkgo.RunSpecs(t, "cfmmvm e2e test suites")
}

func init() {
	flagVars = e2e.RegisterFlags()
}

// Construct tmpnet network with a single MorpheusVM Subnet
var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {
	require := require.New(ginkgo.GinkgoT())

	customAllocs := make([]*genesis.CustomAllocation, 0)
	for _, prefundedAddr := range ed25519Addrs {
		customAllocs = append(customAllocs, &genesis.CustomAllocation{
			Address: prefundedAddr,
			Balance: initialBalance,
		})
	}

	gen := genesis.NewDefaultGenesis(customAllocs)

	genesisBytes, err := json.Marshal(gen)
	require.NoError(err)

	expectedABI, err := abi.NewABI(vm.ActionParser.GetRegisteredTypes(), vm.OutputParser.GetRegisteredTypes())
	require.NoError(err)

	// Import HyperSDK e2e test coverage and inject MorpheusVM name
	// and workload factory to orchestrate the test.
	he2e.SetWorkload(consts.Name, nil, expectedABI)

	tc := e2e.NewTestContext()

	return fixture.NewTestEnvironment(tc, flagVars, owner, consts.Name, consts.ID, genesisBytes).Marshal()
}, func(envBytes []byte) {
	// Run in every ginkgo process

	// Initialize the local test environment from the global state
	e2e.InitSharedTestEnvironment(ginkgo.GinkgoT(), envBytes)
})
