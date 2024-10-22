// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fixture

import (
	"math"
	"time"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/genesis"
)

func newDefaultGenesis(keys []ed25519.PrivateKey, minBlockGap time.Duration) *genesis.DefaultGenesis {
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

	return genesis
}
