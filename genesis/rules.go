// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
)

var _ chain.Rules = (*Rules)(nil)

type Rules struct {
	*Genesis

	networkID uint32
	chainID   ids.ID
}

func New(g *Genesis, networkID uint32, chainID ids.ID) *Rules {
	return &Rules{g, networkID, chainID}
}

func (r *Rules) NetworkID() uint32 {
	return r.networkID
}

func (r *Rules) ChainID() ids.ID {
	return r.chainID
}

func (*Rules) FetchCustom(string) (any, bool) {
	return nil, false
}
