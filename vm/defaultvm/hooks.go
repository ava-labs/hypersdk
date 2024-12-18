// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package defaultvm

import (
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/state/tstate"

	internalfees "github.com/ava-labs/hypersdk/internal/fees"
)

var _ chain.Hooks = (*DefaultHooks)(nil)

type DefaultHooks struct{}

func (DefaultHooks) AfterBlock(_ *tstate.TState, _ uint64) error {
	return nil
}

func (DefaultHooks) AfterTX(
	_ *chain.Transaction,
	_ *chain.Result,
	_ chain.BalanceHandler,
	_ chain.Rules,
	_ *internalfees.Manager,
) error {
	return nil
}
