// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"fmt"
	"math"
	"time"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/hyperevm/crypto/secp256k1"
	"github.com/ava-labs/hypersdk/examples/hyperevm/storage"
	"github.com/ava-labs/hypersdk/fees"

	hgenesis "github.com/ava-labs/hypersdk/genesis"
)

const InitialBalance uint64 = 3_000_000_000_000_000_000

// Each key will have InitialBalance
var (
	ed25519HexKeys = []string{
		"323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7", //nolint:lll
		"8a7be2e0c9a2d09ac2861c34326d6fe5a461d920ba9c2b345ae28e603d517df148735063f8d5d8ba79ea4668358943e5c80bc09e9b2b9a15b5b15db6c1862e88", //nolint:lll
	}

	// 0x64a4871ed5267cC21645ab1a53cC6CAB2D4B27e0
	// 0xc021535Eb43357e6407EDaa1782E579F457AEd6E
	secp256k1Keys = []string{
		"d5fac114a439480fbeb9e831d54173e36b55a1478536b67442c80c31d0eb077d", //nolint:lll
		"779602343b14456eab83837f6fb760cef5bffb66917135802f69e77a8a7750b2", //nolint:lll
	}
)

func New(minBlockGap time.Duration) (*hgenesis.DefaultGenesis, error) {
	authFactories, err := newDefaultAuthFactories()
	if err != nil {
		return nil, err
	}
	// allocate the initial balance to the HyperSDK-native account
	customAllocs := make([]*hgenesis.CustomAllocation, 0, len(authFactories))
	for _, authFactory := range authFactories {
		customAllocs = append(customAllocs, &hgenesis.CustomAllocation{
			Address: authFactory.Address(),
			Balance: InitialBalance,
		})
	}

	// allocate initial balance to EVM-style accounts
	for _, keyHex := range secp256k1Keys {
		bytes, err := codec.LoadHex(keyHex, secp256k1.PrivateKeyLen)
		if err != nil {
			return nil, fmt.Errorf("failed to load key: %w", err)
		}
		pk := secp256k1.PrivateKey(bytes).PublicKey()
		addr, err := secp256k1.PublicKeyToAddress(pk)
		if err != nil {
			return nil, fmt.Errorf("failed to convert public key to address: %w", err)
		}
		customAllocs = append(customAllocs, &hgenesis.CustomAllocation{
			Address: storage.FromEVMAddress(addr),
			Balance: InitialBalance,
		})
	}

	genesis := hgenesis.NewDefaultGenesis(customAllocs)

	// Set WindowTargetUnits to MaxUint64 for all dimensions to iterate full mempool during block building.
	genesis.Rules.WindowTargetUnits = fees.Dimensions{math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64}

	// Set all limits to MaxUint64 to avoid limiting block size for all dimensions except bandwidth. Must limit bandwidth to avoid building
	// a block that exceeds the maximum size allowed by AvalancheGo.
	genesis.Rules.MaxBlockUnits = fees.Dimensions{1800000, math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64}
	genesis.Rules.MinBlockGap = minBlockGap.Milliseconds()

	genesis.Rules.NetworkID = uint32(1)
	genesis.Rules.ChainID = ids.GenerateTestID()

	return genesis, nil
}

func newDefaultAuthFactories() ([]chain.AuthFactory, error) {
	authFactories := make([]chain.AuthFactory, len(ed25519HexKeys))
	for i, keyHex := range ed25519HexKeys {
		bytes, err := codec.LoadHex(keyHex, ed25519.PrivateKeyLen)
		if err != nil {
			return nil, err
		}
		authFactories[i] = auth.NewED25519Factory(ed25519.PrivateKey(bytes))
	}
	return authFactories, nil
}
