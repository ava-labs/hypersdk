// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tieredstorage

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/utils/maybe"
	"github.com/ava-labs/avalanchego/x/merkledb"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/internal/math"
	"github.com/ava-labs/hypersdk/keys"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/shim"
	"github.com/ava-labs/hypersdk/state/tstate"

	safemath "github.com/ava-labs/avalanchego/utils/math"
	internalfees "github.com/ava-labs/hypersdk/internal/fees"
)

var (
	_ shim.Execution  = (*Shim)(nil)
	_ state.Immutable = (*innerShim)(nil)
)

type Shim struct {
	config Config
}

func (s *Shim) ImmutableView(ctx context.Context, stateKeys state.Keys, im state.Immutable, blockHeight uint64) (state.Immutable, error) {
	unsuffixedStorage := make(map[string][]byte)
	hotKeys := make(map[string]uint16)

	for k := range stateKeys {
		v, err := im.GetValue(ctx, []byte(k))
		if err != nil && err != database.ErrNotFound {
			return nil, err
		} else if err == database.ErrNotFound {
			continue
		}

		if len(v) < consts.Uint64Len {
			return nil, errValueTooShortForSuffix
		}

		lastTouched := binary.BigEndian.Uint64(v[len(v)-consts.Uint64Len:])
		memoryThreshold, err := safemath.Sub(blockHeight, s.config.Epsilon)
		if lastTouched >= memoryThreshold || err == safemath.ErrUnderflow {
			maxChunks, ok := keys.MaxChunks([]byte(k))
			if !ok {
				return nil, errFailedToParseMaxChunks
			}
			hotKeys[k] = maxChunks
		}

		unsuffixedStorage[k] = v[:len(v)-consts.Uint64Len]
	}

	return &innerShim{
		hotKeys:   hotKeys,
		Immutable: state.ImmutableStorage(unsuffixedStorage),
		config:    s.config,
	}, nil
}

func (*Shim) MutableView(mu state.Mutable, blockHeight uint64) state.Mutable {
	return state.NewTranslatedMutable(mu, blockHeight)
}

type innerShim struct {
	state.Immutable
	hotKeys map[string]uint16
	config  Config
}

func ExportStateDiff(ctx context.Context, ts *tstate.TState, view state.View, m chain.MetadataManager, blockHeight uint64) (merkledb.View, error) {
	changedKeys := ts.ChangedKeys()
	for k := range changedKeys {
		// Metadata should not be suffixed
		if isMetadataKey(k, m) {
			continue
		}

		if changedKeys[k].HasValue() {
			changedKeys[k] = maybe.Some(
				binary.BigEndian.AppendUint64(changedKeys[k].Value(), blockHeight),
			)
		}
	}

	return view.NewView(ctx, merkledb.ViewChanges{MapOps: changedKeys, ConsumeBytes: true})
}

func ResultModifier(im state.Immutable, result *chain.Result, fm *internalfees.Manager) (*chain.ResultChanges, error) {
	innerShim, ok := im.(*innerShim)
	if !ok {
		return nil, fmt.Errorf("expected innerShim but got %T", im)
	}

	// Compute refund dims
	readsRefundOp := math.NewUint64Operator(0)
	for _, v := range innerShim.hotKeys {
		readsRefundOp.Add(innerShim.config.StorageReadKeyRefund)
		readsRefundOp.MulAdd(uint64(v), innerShim.config.StorageReadValueRefund)
	}
	readRefundUnits, err := readsRefundOp.Value()
	if err != nil {
		return nil, err
	}
	refundDims := fees.Dimensions{0, 0, readRefundUnits, 0, 0}

	// Compute refund fee
	refundFee, err := fm.Fee(refundDims)
	if err != nil {
		return nil, err
	}

	// Modify result
	newDims, err := fees.Sub(result.Units, refundDims)
	if err != nil {
		return nil, err
	}
	result.Units = newDims
	result.Fee -= refundFee

	return &chain.ResultChanges{
		DimsDiff: refundDims,
		FeeDiff:  refundFee,
	}, nil
}

func Refund(ctx context.Context, resultChanges *chain.ResultChanges, bh chain.BalanceHandler, sponsor codec.Address, mu state.Mutable) error {
	return bh.AddBalance(ctx, sponsor, mu, resultChanges.FeeDiff)
}

func FeeManagerModifier(fm *internalfees.Manager, resultChanges *chain.ResultChanges) error {
	ok, _ := fm.Refund(resultChanges.DimsDiff)
	if !ok {
		return errFailedToRefund
	}
	return nil
}

func isMetadataKey(k string, m chain.MetadataManager) bool {
	if bytes.Equal([]byte(k), chain.HeightKey(m.HeightPrefix())) { //nolint:gocritic
		return true
	} else if bytes.Equal([]byte(k), chain.TimestampKey(m.TimestampPrefix())) {
		return true
	} else if bytes.Equal([]byte(k), chain.FeeKey(m.FeePrefix())) {
		return true
	}
	return false
}
