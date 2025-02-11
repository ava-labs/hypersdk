// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package authtest

import (
	"context"
	"math"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
)

type NoAuthFactory struct{}

func (NoAuthFactory) Sign([]byte) (chain.Auth, error) {
	return NoAuth{}, nil
}

func (NoAuthFactory) MaxUnits() (uint64, uint64) {
	return 0, 0
}

func (NoAuthFactory) Address() codec.Address {
	return codec.EmptyAddress
}

type NoAuth struct{}

func (NoAuth) GetTypeID() uint8 {
	return 0
}

func (NoAuth) ValidRange(chain.Rules) (start int64, end int64) {
	return 0, math.MaxInt64
}

func (NoAuth) Marshal(*codec.Packer) {}

func (NoAuth) Size() int { return 0 }

func (NoAuth) ComputeUnits(chain.Rules) uint64 {
	return 0
}

func (NoAuth) Verify(context.Context, []byte) error {
	return nil
}

func (NoAuth) Actor() codec.Address {
	return codec.EmptyAddress
}

func (NoAuth) Sponsor() codec.Address {
	return codec.EmptyAddress
}
