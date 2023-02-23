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
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
	"github.com/ava-labs/hypersdk/utils"
)

const (
	MaxMetadataSize = 256
)

var _ chain.Action = (*Mint)(nil)

type Mint struct {
	// To is the recipient of the [Value].
	To crypto.PublicKey `json:"to"`

	// Metadata
	Metadata []byte `json:"metadata"`

	// Number of assets to mint to [To].
	Value uint64 `json:"value"`
}

func (m *Mint) StateKeys(chain.Auth, ids.ID) [][]byte {
	return [][]byte{
		storage.PrefixAssetKey(m.Asset),
		storage.PrefixBalanceKey(m.To, m.Asset),
	}
}

func (m *Mint) Execute(
	ctx context.Context,
	r chain.Rules,
	db chain.Database,
	_ int64,
	rauth chain.Auth,
	_ ids.ID,
) (*chain.Result, error) {
	actor := auth.GetActor(rauth)
	unitsUsed := m.MaxUnits(r) // max units == units
	if m.Asset == ids.Empty {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputAssetIsNative}, nil
	}
	if m.Value == 0 {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputValueZero}, nil
	}
	owner, err := storage.GetAssetOwner(ctx, db, m.Asset)
	if err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if owner != crypto.EmptyPublicKey {
		return &chain.Result{
			Success: false,
			Units:   unitsUsed,
			Output:  OutputAssetAlreadyExists,
		}, nil
	}
	if err := storage.SetAssetOwner(ctx, db, actor, m.Asset); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if err := storage.SetBalance(ctx, db, m.To, m.Asset, m.Value); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	return &chain.Result{Success: true, Units: unitsUsed}, nil
}

func (*Mint) MaxUnits(chain.Rules) uint64 {
	// We use size as the price of this transaction but we could just as easily
	// use any other calculation.
	return crypto.PublicKeyLen + consts.IDLen + consts.Uint64Len
}

func (m *Mint) Marshal(p *codec.Packer) {
	p.PackPublicKey(m.To)
	p.PackID(m.Asset)
	p.PackUint64(m.Value)
}

func UnmarshalMint(p *codec.Packer) (chain.Action, error) {
	var mint Mint
	p.UnpackPublicKey(&mint.To)
	p.UnpackID(false, &mint.Asset) // empty ID is the native asset
	mint.Value = p.UnpackUint64(true)
	return &mint, p.Err()
}

func (*Mint) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}
