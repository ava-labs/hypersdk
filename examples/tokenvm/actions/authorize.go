// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/genesis"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
)

var _ chain.Action = (*Authorize)(nil)

type Authorize struct {
	// Actor must be specified so we can enumerate read keys
	Actor crypto.PublicKey `json:"actor"`
	// Signer is the new permissions
	//
	// Any balance pull must come from actor to avoid being able to steal other's
	// money.
	Signer crypto.PublicKey `json:"signer"`
	// TODO: based on order of index in permissions
	// if 0, then remove all perms
	ActionPermissions uint8 `json:"actionPermissions"`
	MiscPermissions   uint8 `json:"miscPermissions"`
}

func (a *Authorize) StateKeys(chain.Auth) [][]byte {
	return [][]byte{storage.PrefixPermissionsKey(a.Actor, a.Signer)}
}

func (a *Authorize) Execute(
	ctx context.Context,
	r chain.Rules,
	db chain.Database,
	_ int64,
	rauth chain.Auth,
	_ ids.ID,
) (*chain.Result, error) {
	unitsUsed := a.MaxUnits(r) // max units == units

	// Ensure auth actor is the same as actor specified in tx.
	actor := auth.GetActor(rauth)
	if actor != a.Actor {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputActorMismatch}, nil
	}

	// Ensure permissions actually do something
	actionPermissions, miscPermissions, err := storage.GetPermissions(ctx, db, a.Actor, a.Signer)
	if err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if actionPermissions == a.ActionPermissions && miscPermissions == a.MiscPermissions {
		return &chain.Result{
			Success: false,
			Units:   unitsUsed,
			Output:  OutputPermissionsUseless,
		}, nil
	}
	stateLockup, err := genesis.GetStateLockup(r)
	if err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	switch {
	case actionPermissions == 0 && miscPermissions == 0:
		// need to pay state lockup
		if err := storage.LockBalance(ctx, db, a.Actor, stateLockup); err != nil { // new accounts must lock funds
			// TODO: where should lock funds?
			return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
		}
		if err := storage.SetPermissions(ctx, db, a.Actor, a.Signer, a.ActionPermissions, a.MiscPermissions); err != nil {
			return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
		}
	case a.ActionPermissions == 0 && a.MiscPermissions == 0:
		// get refund
		if err := storage.UnlockBalance(ctx, db, a.Actor, stateLockup); err != nil {
			return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
		}
		if err := storage.DeletePermissions(ctx, db, a.Actor, a.Signer); err != nil {
			return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
		}
	default:
		// Simple override of permissions
		if err := storage.SetPermissions(ctx, db, a.Actor, a.Signer, a.ActionPermissions, a.MiscPermissions); err != nil {
			return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
		}
	}
	return &chain.Result{Success: true, Units: unitsUsed}, nil
}

func (*Authorize) MaxUnits(chain.Rules) uint64 {
	// TODO: add a "state touch" constant based on number of times touching state
	// minUnits == size, maxUnits == size + max state touches
	return crypto.PublicKeyLen*2 + 2
}

func (a *Authorize) Marshal(p *codec.Packer) {
	p.PackPublicKey(a.Actor)
	p.PackPublicKey(a.Signer)
	p.PackByte(a.ActionPermissions)
	p.PackByte(a.MiscPermissions)
}

func UnmarshalAuthorize(p *codec.Packer) (chain.Action, error) {
	var authorize Authorize
	p.UnpackPublicKey(&authorize.Actor)
	p.UnpackPublicKey(&authorize.Signer)
	authorize.ActionPermissions = p.UnpackByte()
	authorize.MiscPermissions = p.UnpackByte()
	return &authorize, p.Err()
}

func (*Authorize) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}
