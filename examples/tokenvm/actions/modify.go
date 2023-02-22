// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
)

var _ chain.Action = (*Modify)(nil)

// Modify will update the Royalty of any claimed content or claim unclaimed
// content
//
// This allows content creators to update previously indexed info without
// unindexing im.
type Modify struct {
	// Content is the content to update
	Content ids.ID `json:"content"`

	// Royaly is the new value to apply to the content
	Royalty uint64 `json:"royalty"`
}

func (m *Modify) StateKeys(chain.Auth) [][]byte {
	return [][]byte{storage.PrefixContentKey(m.Content)}
}

func (m *Modify) Execute(
	ctx context.Context,
	r chain.Rules,
	db chain.Database,
	_ int64,
	rauth chain.Auth,
	_ ids.ID,
) (*chain.Result, error) {
	actor := auth.GetActor(rauth)
	unitsUsed := m.MaxUnits(r) // max units == units

	if m.Content == ids.Empty {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputContentEmpty}, nil
	}
	if m.Royalty == 0 {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputInvalidObject}, nil
	}
	owner, oldRoyalty, exists, err := storage.GetContent(ctx, db, m.Content)
	if err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if !exists {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputContentMissing}, nil
	}
	if owner != actor {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputWrongOwner}, nil
	}
	if oldRoyalty == m.Royalty {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputInvalidObject}, nil
	}
	if err := storage.SetContent(ctx, db, m.Content, actor, m.Royalty); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	return &chain.Result{Success: true, Units: unitsUsed}, nil
}

func (*Modify) MaxUnits(chain.Rules) uint64 {
	return consts.IDLen + consts.Uint64Len
}

func (m *Modify) Marshal(p *codec.Packer) {
	p.PackID(m.Content)
	p.PackUint64(m.Royalty)
}

func UnmarshalModify(p *codec.Packer) (chain.Action, error) {
	var modify Modify
	p.UnpackID(true, &modify.Content)
	modify.Royalty = p.UnpackUint64(true)
	return &modify, p.Err()
}

func (*Modify) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}
