// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tieredstorage

import (
	"context"
	"encoding/binary"

	"github.com/ava-labs/avalanchego/utils/maybe"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/internal/math"
	"github.com/ava-labs/hypersdk/keys"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"

	safemath "github.com/ava-labs/avalanchego/utils/math"
	internalfees "github.com/ava-labs/hypersdk/internal/fees"
)

var _ chain.TransactionManager = (*TieredStorageTransactionManager)(nil)

type TieredStorageTransactionManager struct {
	config  Config
	hotKeys map[string]uint16
}

func newTieredStorageTransactionManager(config Config) *TieredStorageTransactionManager {
	return &TieredStorageTransactionManager{
		config: config,
	}
}

// MutableView implements chain.TransactionManager.
func (*TieredStorageTransactionManager) MutableView(mu state.Mutable, blockHeight uint64) state.Mutable {
	return state.NewTranslatedMutable(mu, blockHeight)
}

func (*TieredStorageTransactionManager) ImmutableView(im state.Immutable) state.Immutable {
	return state.NewTranslatedImmutable(im)
}

// AfterTX issues fee refunds and, if the validator is calling, unit refunds
func (t *TieredStorageTransactionManager) AfterTX(
	ctx context.Context,
	tx *chain.Transaction,
	result *chain.Result,
	mu state.Mutable,
	bh chain.BalanceHandler,
	fm *internalfees.Manager,
	isBuilder bool,
) error {
	// We deduct fees regardless of whether the builder or processor is calling
	readsRefundOp := math.NewUint64Operator(0)
	for _, v := range t.hotKeys {
		readsRefundOp.Add(t.config.StorageReadKeyRefund)
		readsRefundOp.MulAdd(uint64(v), t.config.StorageReadValueRefund)
	}
	readRefundUnits, err := readsRefundOp.Value()
	if err != nil {
		return err
	}

	refundDims := fees.Dimensions{0, 0, readRefundUnits, 0, 0}
	refundFee, err := fm.Fee(refundDims)
	if err != nil {
		return err
	}
	if err := bh.AddBalance(ctx, tx.Auth.Sponsor(), mu, refundFee); err != nil {
		return err
	}

	if !isBuilder {
		if err := t.refundUnitsConsumed(fm, refundDims); err != nil {
			return err
		}
	}

	newUnits, err := fees.Sub(result.Units, refundDims)
	if err != nil {
		return err
	}

	result.Units = newUnits
	result.Fee -= refundFee

	return nil
}

// ExecutableState implements chain.TransactionManager.
func (t *TieredStorageTransactionManager) ExecutableState(
	mp map[string][]byte,
	blockHeight uint64,
) (state.Immutable, error) {
	unsuffixedStorage := make(map[string][]byte)
	hotKeys := make(map[string]uint16)

	for k, v := range mp {
		if len(v) < consts.Uint64Len {
			return nil, errValueTooShortForSuffix
		}

		lastTouched := binary.BigEndian.Uint64(v[len(v)-consts.Uint64Len:])

		memoryThreshold, err := safemath.Sub(blockHeight, t.config.Epsilon)
		if lastTouched >= memoryThreshold || err == safemath.ErrUnderflow {
			maxChunks, ok := keys.MaxChunks([]byte(k))
			if !ok {
				return nil, errFailedToParseMaxChunks
			}
			hotKeys[k] = maxChunks
		}

		unsuffixedStorage[k] = v[:len(v)-consts.Uint64Len]
	}

	t.hotKeys = hotKeys
	return tstate.ImmutableScopeStorage(unsuffixedStorage), nil
}

func (*TieredStorageTransactionManager) RawState(ts *tstate.TState, blockHeight uint64) error {
	pendingChangedKeys := ts.ChangedKeys()
	for k := range pendingChangedKeys {
		if pendingChangedKeys[k].HasValue() {
			pendingChangedKeys[k] = maybe.Some(
				binary.BigEndian.AppendUint64(
					pendingChangedKeys[k].Value(),
					blockHeight,
				),
			)
		}
	}

	ts.SetChangedKeys(pendingChangedKeys)
	return nil
}

func (*TieredStorageTransactionManager) refundUnitsConsumed(
	fm *internalfees.Manager,
	refundDims fees.Dimensions,
) error {
	ok, _ := fm.Refund(refundDims)
	if !ok {
		return errFailedToRefund
	}
	return nil
}

func (t *TieredStorageTransactionManager) IsHotKey(key string) bool {
	_, ok := t.hotKeys[key]
	return ok
}
