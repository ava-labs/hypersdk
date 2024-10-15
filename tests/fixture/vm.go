// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fixture

import (
	"encoding/json"
	"time"

	"github.com/ava-labs/hypersdk/genesis"
)

type testVM struct {
	// every VM has a genesis. We initialize the VM with a default genesis.
	genesis *genesis.DefaultGenesis
	// pre-set keys on the vm
	keys []*Ed25519TestKey
}

func NewTestVM(minBlockGap time.Duration) *testVM {
	keys := newDefualtKeys()
	genesis := newDefaultGenesis(keys, minBlockGap)
	return &testVM{
		genesis: genesis,
		keys:    keys,
	}
}

func (t *testVM) GetGenesisBytes() ([]byte, error) {
	return json.Marshal(t.genesis)
}

func (t *testVM) GetKeys() []*Ed25519TestKey {
	return t.keys
}
