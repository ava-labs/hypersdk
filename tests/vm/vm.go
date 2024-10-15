// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"encoding/json"
	"time"

	"github.com/ava-labs/hypersdk/abi"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/genesis"
)

type VM struct {
	// every VM has a genesis. We initialize the VM with a default genesis.
	genesis *genesis.DefaultGenesis
	// pre-set keys on the vm
	Keys []*Ed25519TestKey
	// expectedABI
	expectedABI abi.ABI
	// parser
	parser chain.Parser
}

func NewVM(minBlockGap time.Duration) *VM {
	keys := newDefualtKeys()
	genesis := newDefaultGenesis(keys, minBlockGap)
	return &VM{
		genesis: genesis,
		Keys:    keys,
	}
}

func (v *VM) GetGenesisBytes() ([]byte, error) {
	return json.Marshal(v.genesis)
}
