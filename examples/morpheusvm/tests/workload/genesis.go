// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package workload

import (
	"encoding/json"
	"math"
	"time"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/vm"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/tests/workload"
)

const (
	// default initial balance for each address
	InitialBalance uint64 = 3_000_000_000_000_000_000
)

// hardcoded initial set of ed25519 keys. Each will be initialized with InitialBalance
var ed25519HexKeys = []string{
	"323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7", //nolint:lll
	"8a7be2e0c9a2d09ac2861c34326d6fe5a461d920ba9c2b345ae28e603d517df148735063f8d5d8ba79ea4668358943e5c80bc09e9b2b9a15b5b15db6c1862e88", //nolint:lll
}

func newGenesis(authFactories []chain.AuthFactory, minBlockGap time.Duration) *genesis.DefaultGenesis {
	// allocate the initial balance to the addresses
	customAllocs := make([]*genesis.CustomAllocation, 0, len(authFactories))
	for _, authFactory := range authFactories {
		customAllocs = append(customAllocs, &genesis.CustomAllocation{
			Address: authFactory.Address(),
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

func newDefaultAuthFactories() []chain.AuthFactory {
	authFactories := make([]chain.AuthFactory, len(ed25519HexKeys))
	for i, keyHex := range ed25519HexKeys {
		bytes, err := codec.LoadHex(keyHex, ed25519.PrivateKeyLen)
		if err != nil {
			panic(err)
		}
		authFactories[i] = auth.NewED25519Factory(ed25519.PrivateKey(bytes))
	}
	return authFactories
}

func NewTestNetworkConfig(minBlockGap time.Duration) (workload.DefaultTestNetworkConfiguration, error) {
	keys := newDefaultAuthFactories()
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
