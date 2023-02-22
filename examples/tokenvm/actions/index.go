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

var _ chain.Action = (*Index)(nil)

const (
	// We restrict the [MaxContentSize] to be 768B such that any collection of
	// 1024 keys+values will never be more than [chain.NetworkSizeLimit].
	//
	// TODO: relax this once merkleDB/sync can ensure range proof resolution does
	// not surpass [NetworkSizeLimit]
	MaxContentSize = 1_600
)

type Index struct {
	// REQUIRED
	//
	// Schema of the content being indexed
	Schema ids.ID `json:"schema"`
	// Content is the indexed data that will be associated with the ID
	Content []byte `json:"content"`

	// OPTIONAL
	//
	// Royalty is the amount required to reference this object as a parent in
	// another [Index]
	//
	// If this value is > 0, the content will be registered to receive rewards
	// and the creator will need to lock up [Genesis.ContentStake]. To deregister an item
	// from receiving rewards and to receive their [Genesis.ContentStake] back, the creator must
	// issue an [UnindexTx].
	//
	// If this value is 0, the content will not be registered to receive rewards.
	Royalty uint64 `json:"royalty"`
	// Parent of the content being indexed (this may be nested)
	//
	// This can also be empty if there is no parent (first reference)
	Parent ids.ID `json:"parent"`
	// Searcher is the owner of [Parent]
	//
	// We require this in the transaction so that the owner can be prefetched
	// during execution.
	Searcher crypto.PublicKey `json:"searcher"`
	// Servicer is the recipient of the [Invoice] payment
	//
	// This is not enforced anywhere on-chain and is up to the transaction signer
	// to populate correctly. If not populate correctly, it is likely that the
	// service provider will simply stop serving the user.
	Servicer crypto.PublicKey `json:"servicer"`
	// Commission is the value to send to [Servicer] for their work in surfacing
	// the content for interaction
	//
	// This field is not standardized and enforced by a [Servicer] to provide
	// user-level flexibility. For example, a [Servicer] may choose to offer
	// a discount after performing so many interactions per month.
	Commission uint64 `json:"commission"`
}

func (i *Index) StateKeys(rauth chain.Auth) [][]byte {
	actor := auth.GetActor(rauth)
	keys := [][]byte{storage.PrefixBalanceKey(actor)}
	if i.Parent != ids.Empty {
		keys = append(keys, storage.PrefixContentKey(i.Parent))
		if i.Searcher != crypto.EmptyPublicKey {
			keys = append(keys, storage.PrefixBalanceKey(i.Searcher))
		}
	}
	if i.Royalty > 0 {
		keys = append(keys, storage.PrefixContentKey(i.ContentID()))
	}
	if i.Servicer != crypto.EmptyPublicKey && i.Commission > 0 {
		// You can be serviced with or without a [Parent]
		keys = append(keys, storage.PrefixBalanceKey(i.Servicer))
	}
	return keys
}

func (i *Index) Execute(
	ctx context.Context,
	r chain.Rules,
	db chain.Database,
	_ int64,
	rauth chain.Auth,
	_ ids.ID,
) (*chain.Result, error) {
	actor := auth.GetActor(rauth)
	unitsUsed := i.MaxUnits(r) // max units == units

	if i.Schema == ids.Empty {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputInvalidSchema}, nil
	}
	if len(i.Content) > MaxContentSize {
		// This should already be caught by encoder but we check anyways.
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputInvalidContent}, nil
	}
	if i.Parent != ids.Empty {
		owner, err := storage.RewardSearcher(ctx, db, i.Parent, actor)
		if err != nil { // no-op if not indexed
			return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
		}
		if owner != i.Searcher {
			// This will also be checked by key access during block execution.
			return &chain.Result{Success: false, Units: unitsUsed, Output: OutputWrongOwner}, nil
		}
	} else { //nolint:gocritic
		if i.Searcher != crypto.EmptyPublicKey {
			// If no [Parent], can't name a valid searcher.
			return &chain.Result{Success: false, Units: unitsUsed, Output: OutputInvalidObject}, nil
		}
	}
	if i.Royalty > 0 {
		// It is ok to charge royalties on objects that have parents.
		if err := storage.IndexContent(ctx, db, i.ContentID(), actor, i.Royalty); err != nil { // will fail if already indexed
			return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
		}
		stateLockup, err := genesis.GetStateLockup(r)
		if err != nil {
			return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
		}
		if err := storage.LockBalance(ctx, db, actor, stateLockup); err != nil {
			return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
		}
	}
	if i.Servicer == crypto.EmptyPublicKey {
		if i.Commission > 0 {
			return &chain.Result{Success: false, Units: unitsUsed, Output: OutputInvalidObject}, nil
		}
		return &chain.Result{Success: true, Units: unitsUsed}, nil
	}
	// It is ok to pay 0 commission on an invoice.
	if i.Commission == 0 {
		return &chain.Result{Success: true, Units: unitsUsed}, nil
	}
	if err := storage.SubUnlockedBalance(ctx, db, actor, i.Commission); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	exists, err := storage.AddUnlockedBalance(ctx, db, i.Servicer, i.Commission, false)
	if err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if !exists {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputServicerMissing}, nil
	}
	return &chain.Result{Success: true, Units: unitsUsed}, nil
}

func (i *Index) MaxUnits(chain.Rules) uint64 {
	totalSize := uint64(consts.IDLen + consts.IntLen + len(i.Content) + 1 /* optional flag */)
	if i.Royalty > 0 {
		totalSize += consts.Uint64Len
	}
	if i.Parent != ids.Empty {
		totalSize += consts.IDLen
	}
	if i.Searcher != crypto.EmptyPublicKey {
		totalSize += crypto.PublicKeyLen
	}
	if i.Servicer != crypto.EmptyPublicKey {
		totalSize += crypto.PublicKeyLen
	}
	if i.Commission > 0 {
		totalSize += consts.Uint64Len
	}
	return totalSize
}

// ContentID is the canonical identifier of the indexed data. We include the
// minimum amount of info required to describe the item.
func (i *Index) ContentID() ids.ID {
	p := codec.NewWriter(consts.MaxInt)
	p.PackID(i.Parent)
	p.PackBytes(i.Content)
	return utils.ToID(p.Bytes())
}

// Marshal encodes all fields into the provided packer
func (i *Index) Marshal(p *codec.Packer) {
	// Required
	p.PackID(i.Schema)
	p.PackBytes(i.Content)

	// Optional
	op := codec.NewOptionalWriter()
	op.PackUint64(i.Royalty)
	op.PackID(i.Parent)
	op.PackPublicKey(i.Searcher)
	op.PackPublicKey(i.Servicer)
	op.PackUint64(i.Commission)
	p.PackOptional(op)
}

func UnmarshalIndex(p *codec.Packer) (chain.Action, error) {
	var index Index

	// Required
	p.UnpackID(true, &index.Schema)
	p.UnpackBytes(MaxContentSize, true, &index.Content)

	// Optional
	op := p.NewOptionalReader()
	index.Royalty = op.UnpackUint64()
	op.UnpackID(&index.Parent)
	op.UnpackPublicKey(&index.Searcher)
	op.UnpackPublicKey(&index.Servicer)
	index.Commission = op.UnpackUint64()
	if err := p.Err(); err != nil {
		return nil, err
	}
	return &index, nil
}

func (*Index) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}
