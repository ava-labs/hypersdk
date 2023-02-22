// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/genesis"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
)

var _ chain.Action = (*Clear)(nil)

type Clear struct {
	// To is the recipient of [Actor]'s funds
	To crypto.PublicKey `json:"to"`
}

func (c *Clear) StateKeys(rauth chain.Auth) [][]byte {
	actor := auth.GetActor(rauth)
	return [][]byte{storage.PrefixBalanceKey(actor), storage.PrefixBalanceKey(c.To)}
}

func (c *Clear) Execute(
	ctx context.Context,
	r chain.Rules,
	db chain.Database,
	_ int64,
	rauth chain.Auth,
	_ ids.ID,
) (*chain.Result, error) {
	actor := auth.GetActor(rauth)
	unitsUsed := c.MaxUnits(r) // max units == units

	stateLockup, err := genesis.GetStateLockup(r)
	if err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	u, l, err := storage.GetBalance(ctx, db, actor)
	if err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if l == 0 {
		return &chain.Result{
			Success: false,
			Units:   unitsUsed,
			Output:  OutputInvalidBalance,
		}, nil
	}
	// Ensure all items are deleted before an account is
	if l != stateLockup*2 /* balance + single signer permissions */ {
		// Invariant: this can never be less than this at this point, if only
		// balance remained, this could not be executed
		return &chain.Result{
			Success: false,
			Units:   unitsUsed,
			Output:  OutputAccountNotEmpty,
		}, nil
	}
	transferAmount := u + l
	if err := storage.DeleteBalance(ctx, db, actor); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if err := storage.DeletePermissions(ctx, db, actor, auth.GetSigner(rauth)); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	alreadyExists, err := storage.AddUnlockedBalance(ctx, db, c.To, transferAmount, false)
	if err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if alreadyExists {
		return &chain.Result{Success: true, Units: unitsUsed}, nil
	}
	// new accounts must lock funds for balance and perms
	if err := storage.LockBalance(ctx, db, c.To, stateLockup*2); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	// new accounts have default perms
	if err := storage.SetPermissions(ctx, db, c.To, c.To, consts.MaxUint8, consts.MaxUint8); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	return &chain.Result{Success: true, Units: unitsUsed}, nil
}

func (*Clear) MaxUnits(chain.Rules) uint64 {
	return crypto.PublicKeyLen
}

func (c *Clear) Marshal(p *codec.Packer) {
	p.PackPublicKey(c.To)
}

func UnmarshalClear(p *codec.Packer) (chain.Action, error) {
	var transfer Clear
	p.UnpackPublicKey(&transfer.To)
	return &transfer, p.Err()
}

func (*Clear) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}
