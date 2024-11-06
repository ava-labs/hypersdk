// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"
)

func TestCallAction(t *testing.T) {
	ts := tstate.New(1)
	addr := codec.CreateAddress(0, ids.GenerateTestID())
	tests := []chaintest.ActionTest{
		{
			Name:  "No Statekeys",
			Actor: codec.EmptyAddress,
			Action: &Call{
				ContractAddress: addr,
				Value:           0,
			},
			State:       ts.NewView(make(state.Keys), map[string][]byte{}),
			ExpectedErr: tstate.ErrInvalidKeyOrPermission,
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}
