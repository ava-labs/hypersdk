// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

//go:generate go run github.com/StephenButtolph/canoto/canoto $GOFILE

import (
	"fmt"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
)

const (
	// Timestamp (varint) + ChainID (size + fixed 32 bytes) + MaxFee (fixed uint64) + 3 wire tags
	MaxBaseSize = consts.MaxVarintLen + consts.Uint64Len + ids.IDLen + consts.MaxVarintLen + 3*consts.MaxVarintLen
	// TODO: make this divisor configurable
	validityWindowTimestampDivisor = consts.MillisecondsPerSecond
)

type Base struct {
	// Timestamp is the expiry of the transaction (inclusive). Once this time passes and the
	// transaction is not included in a block, it is safe to regenerate it.
	Timestamp int64 `canoto:"int,1" json:"timestamp"`

	// ChainID protects against replay attacks on different VM instances.
	ChainID ids.ID `canoto:"fixed bytes,2" json:"chainId"`

	// MaxFee is the max fee the user will pay for the transaction to be executed. The chain
	// will charge anything up to this price if the transaction makes it on-chain.
	//
	// If the fee is too low to pay all fees, the transaction will be dropped.
	MaxFee uint64 `canoto:"fint64,3" json:"maxFee"`

	// Base uses "noatomic" tag to ensure it can be safely passed by value (immutable)
	canotoData canotoData_Base `canoto:"noatomic"` //nolint:revive
}

func (b *Base) Execute(r Rules, timestamp int64) error {
	if b.ChainID != r.GetChainID() {
		return fmt.Errorf("%w: chainID=%s, expected=%s", ErrInvalidChainID, b.ChainID, r.GetChainID())
	}
	return validitywindow.VerifyTimestamp(b.Timestamp, timestamp, validityWindowTimestampDivisor, r.GetValidityWindow())
}
