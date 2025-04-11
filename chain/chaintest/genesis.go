// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chaintest

import (
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/genesis"
)

func CreateGenesis(
	numAccounts uint64,
	allocAmount uint64,
	factoryF func() (chain.AuthFactory, error),
) ([]chain.AuthFactory, genesis.Genesis, error) {
	factories := make([]chain.AuthFactory, numAccounts)
	customAllocs := make([]*genesis.CustomAllocation, numAccounts)
	for i := range numAccounts {
		factory, err := factoryF()
		if err != nil {
			return nil, nil, err
		}
		factories[i] = factory
		customAllocs[i] = &genesis.CustomAllocation{
			Address: factory.Address(),
			Balance: allocAmount,
		}
	}
	return factories, genesis.NewDefaultGenesis(customAllocs), nil
}

func ED25519Factory() (chain.AuthFactory, error) {
	pk, err := ed25519.GeneratePrivateKey()
	if err != nil {
		return nil, err
	}
	return auth.NewED25519Factory(pk), nil
}
