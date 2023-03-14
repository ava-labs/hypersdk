// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
	"github.com/ava-labs/hypersdk/utils"
)

var _ chain.Action = (*CreateAsset)(nil)

type CreateAsset struct {
	// Metadata is creator-specified information about the asset. This can be
	// modified using the [ModifyAsset] action.
	Metadata []byte `json:"metadata"`
}

func (*CreateAsset) StateKeys(_ chain.Auth, txID ids.ID) [][]byte {
	return [][]byte{storage.PrefixAssetKey(txID)}
}

func (c *CreateAsset) Execute(
	ctx context.Context,
	r chain.Rules,
	db chain.Database,
	_ int64,
	rauth chain.Auth,
	txID ids.ID,
	_ bool,
) (*chain.Result, error) {
	actor := auth.GetActor(rauth)
	unitsUsed := c.MaxUnits(r) // max units == units
	if len(c.Metadata) > MaxMetadataSize {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputMetadataTooLarge}, nil
	}
	// It should only be possible to overwrite an existing asset if there is
	// a hash collision.
	if err := storage.SetAsset(ctx, db, txID, c.Metadata, 0, actor, false); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	return &chain.Result{Success: true, Units: unitsUsed}, nil
}

func (c *CreateAsset) MaxUnits(chain.Rules) uint64 {
	// We use size as the price of this transaction but we could just as easily
	// use any other calculation.
	return uint64(len(c.Metadata))
}

func (c *CreateAsset) Marshal(p *codec.Packer) {
	p.PackBytes(c.Metadata)
}

func UnmarshalCreateAsset(p *codec.Packer, _ *warp.Message) (chain.Action, error) {
	var create CreateAsset
	p.UnpackBytes(MaxMetadataSize, false, &create.Metadata)
	return &create, p.Err()
}

func (*CreateAsset) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}
