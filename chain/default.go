// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"

	"github.com/ava-labs/avalanchego/x/merkledb"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"

	internalfees "github.com/ava-labs/hypersdk/internal/fees"
)

func DefaultExportStateDiff(ctx context.Context, ts *tstate.TState, view state.View, _ MetadataManager, _ uint64) (merkledb.View, error) {
	return view.NewView(ctx, merkledb.ViewChanges{MapOps: ts.ChangedKeys(), ConsumeBytes: true})
}

// This modifies the result passed in
// Futhermore, any changes that were made in Result should be documented in ResultChanges
func DefaultResultModifier(state.Immutable, *Result, *internalfees.Manager) (*ResultChanges, error) {
	return nil, nil
}

func DefaultRefundFunc(context.Context, *ResultChanges, BalanceHandler, codec.Address, state.Mutable) error {
	return nil
}

func FeeManagerModifier(*internalfees.Manager, *ResultChanges) error {
	return nil
}
