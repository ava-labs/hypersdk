// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package authtest

import (
	"context"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
)

type MockAuth struct {
	Start             int64
	End               int64
	ComputeUnitsValue uint64
	ActorAddr         codec.Address
	SponsorAddr       codec.Address
	VerifyError       error
}

func (m *MockAuth) Actor() codec.Address {
	return m.ActorAddr
}

func (m *MockAuth) ComputeUnits(chain.Rules) uint64 {
	return m.ComputeUnitsValue
}

func (*MockAuth) GetTypeID() uint8 {
	panic("unimplemented")
}

func (*MockAuth) Marshal(*codec.Packer) {
	panic("unimplemented")
}

func (*MockAuth) Size() int {
	panic("unimplemented")
}

func (m *MockAuth) Sponsor() codec.Address {
	return m.SponsorAddr
}

func (m *MockAuth) ValidRange(chain.Rules) (int64, int64) {
	return m.Start, m.End
}

func (m *MockAuth) Verify(context.Context, []byte) error {
	return m.VerifyError
}
