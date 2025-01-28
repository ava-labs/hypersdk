// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"fmt"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
)

const (
	BaseSize = consts.Uint64Len*2 + ids.IDLen
	// TODO: make this divisor configurable
	validityWindowTimestampDivisor = consts.MillisecondsPerSecond
)

type Base struct {
	// Timestamp is the expiry of the transaction (inclusive). Once this time passes and the
	// transaction is not included in a block, it is safe to regenerate it.
	Timestamp int64 `json:"timestamp"`

	// ChainID protects against replay attacks on different VM instances.
	ChainID ids.ID `json:"chainId"`

	// MaxFee is the max fee the user will pay for the transaction to be executed. The chain
	// will charge anything up to this price if the transaction makes it on-chain.
	//
	// If the fee is too low to pay all fees, the transaction will be dropped.
	MaxFee uint64 `json:"maxFee"`
}

func (b *Base) Execute(r Rules, timestamp int64) error {
	if b.ChainID != r.GetChainID() {
		return fmt.Errorf("%w: chainID=%s, expected=%s", ErrInvalidChainID, b.ChainID, r.GetChainID())
	}
	return validitywindow.VerifyTimestamp(b.Timestamp, timestamp, validityWindowTimestampDivisor, r.GetValidityWindow())
}

func (*Base) Size() int {
	return BaseSize
}

func (b *Base) Marshal(p *codec.Packer) {
	p.PackInt64(b.Timestamp)
	p.PackID(b.ChainID)
	p.PackUint64(b.MaxFee)
}

// UnmarshalBase unmarshals a Base from packer.
// Caller can assume packer errors are returned from the function.
func UnmarshalBase(p *codec.Packer) (*Base, error) {
	var base Base
	base.Timestamp = p.UnpackInt64(true)
	if base.Timestamp%consts.MillisecondsPerSecond != 0 {
		// TODO: make this modulus configurable
		return nil, fmt.Errorf("%w: timestamp=%d", validitywindow.ErrMisalignedTime, base.Timestamp)
	}
	p.UnpackID(true, &base.ChainID)
	base.MaxFee = p.UnpackUint64(true)
	return &base, p.Err()
}
