// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/hypersdk/chain"
)

// GenesisWithInitialRules is used primarily for tests to create the genesis bytes
type GenesisWithInitialRules[G Genesis, R chain.Rules] struct {
	Genesis      G `json:"genesis"`
	InitialRules R `json:"rules"`
}
