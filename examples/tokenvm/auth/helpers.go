// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package auth

import (
	"context"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
	"github.com/ava-labs/hypersdk/examples/tokenvm/utils"
)

func GetActor(auth chain.Auth) crypto.PublicKey {
	switch a := auth.(type) {
	case *Direct:
		return a.Signer
	case *Delegate:
		return a.Actor
	default:
		return crypto.EmptyPublicKey
	}
}

func GetSigner(auth chain.Auth) crypto.PublicKey {
	switch a := auth.(type) {
	case *Direct:
		return a.Signer
	case *Delegate:
		return a.Signer
	default:
		return crypto.EmptyPublicKey
	}
}

func Authorized(
	ctx context.Context,
	db chain.Database,
	action chain.Action,
	actor crypto.PublicKey,
	signer crypto.PublicKey,
	actorPays bool,
) error {
	actionPerms, miscPerms, err := storage.GetPermissions(ctx, db, actor, signer)
	if err != nil {
		return err
	}
	index, _, exists := consts.ActionRegistry.LookupType(action)
	if !exists {
		return ErrActionMissing
	}
	if !utils.CheckBit(actionPerms, index) {
		return ErrNotAllowed
	}
	if actorPays && !utils.CheckBit(miscPerms, actorPaysBit) {
		return ErrNotAllowed
	}
	return nil
}
