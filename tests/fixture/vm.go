// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fixture

import (
	"encoding/json"
	"time"

	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/genesis"
)

type testVM struct {
	// every VM has a genesis. We initialize the VM with a default genesis.
	genesis *genesis.DefaultGenesis
	// pre-set keys on the vm
	keys []ed25519.PrivateKey
}

func NewTestVM(minBlockGap time.Duration) *testVM {
	keys := newDefaultKeys()
	genesis := newDefaultGenesis(keys, minBlockGap)
	return &testVM{
		genesis: genesis,
		keys:    keys,
	}
}

func (t *testVM) GetGenesisBytes() ([]byte, error) {
	return json.Marshal(t.genesis)
}

func (t *testVM) GetKeys() []ed25519.PrivateKey {
	return t.keys
}
