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
	"github.com/ava-labs/hypersdk/examples/tokenvm/genesis"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
)

var _ chain.Action = (*Unindex)(nil)

type Unindex struct {
	// Content is the content to unindex
	//
	// This transaction will refund [Genesis.ContentStake] to the creator of the
	// content.
	Content ids.ID `json:"content"`
}

func (u *Unindex) StateKeys(rauth chain.Auth) [][]byte {
	actor := auth.GetActor(rauth)
	return [][]byte{storage.PrefixBalanceKey(actor), storage.PrefixContentKey(u.Content)}
}

func (u *Unindex) Execute(
	ctx context.Context,
	r chain.Rules,
	db chain.Database,
	_ int64,
	rauth chain.Auth,
	_ ids.ID,
) (*chain.Result, error) {
	actor := auth.GetActor(rauth)
	unitsUsed := u.MaxUnits(r) // max units == units

	if u.Content == ids.Empty {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputInvalidContent}, nil
	}
	if err := storage.UnindexContent(ctx, db, u.Content, actor); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	stateLockup, err := genesis.GetStateLockup(r)
	if err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if err := storage.UnlockBalance(ctx, db, actor, stateLockup); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	return &chain.Result{Success: true, Units: unitsUsed}, nil
}

func (*Unindex) MaxUnits(chain.Rules) uint64 {
	return consts.IDLen
}

func (u *Unindex) Marshal(p *codec.Packer) {
	p.PackID(u.Content)
}

func UnmarshalUnindex(p *codec.Packer) (chain.Action, error) {
	var unindex Unindex
	p.UnpackID(true, &unindex.Content)
	return &unindex, p.Err()
}

func (*Unindex) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}
