// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
)

var _ (chain.StateManager) = (*StateManager)(nil)

type StateManager struct{}

func (*StateManager) HeightKey() string {
	return HeightKey()
}

func (*StateManager) PHeightKey() string {
	return PHeightKey()
}

func (*StateManager) TimestampKey() string {
	return TimestampKey()
}

func (*StateManager) IncomingWarpKeyPrefix(sourceChainID ids.ID, msgID ids.ID) string {
	return IncomingWarpKeyPrefix(sourceChainID, msgID)
}

func (*StateManager) OutgoingWarpKeyPrefix(txID ids.ID) string {
	return OutgoingWarpKeyPrefix(txID)
}

func (*StateManager) SponsorStateKeys(addr codec.Address) state.Keys {
	return state.Keys{
		string(BalanceKey(addr)): state.Read | state.Write,
	}
}

func (*StateManager) CanDeduct(
	ctx context.Context,
	addr codec.Address,
	im state.Immutable,
	amount uint64,
) (bool, error) {
	bal, err := GetBalance(ctx, im, addr)
	if err != nil {
		return false, err
	}
	return bal >= amount, nil
}

func (*StateManager) Deduct(
	ctx context.Context,
	addr codec.Address,
	mu state.Mutable,
	amount uint64,
) error {
	return SubBalance(ctx, mu, addr, amount)
}

func (*StateManager) EpochKey(epoch uint64) string {
	return EpochKey(epoch)
}
