// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"

	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/shim"

	internalfees "github.com/ava-labs/hypersdk/internal/fees"
)

var (
	_ Hooks              = (*DefaultTransactionHooks)(nil)
	_ TransactionManager = (*DefaultTransactionManager)(nil)
)

type DefaultTransactionHooks struct{}

func (*DefaultTransactionHooks) AfterTX(
	_ context.Context,
	_ *Transaction,
	_ *Result,
	_ state.Mutable,
	_ BalanceHandler,
	_ *internalfees.Manager,
	_ bool,
) error {
	return nil
}

type DefaultTransactionManager struct {
	shim.NoOp
	DefaultTransactionHooks
}

func NewDefaultTransactionManager() TransactionManager {
	return &DefaultTransactionManager{}
}

func DefaultTransactionManagerFactory() TransactionManagerFactory {
	return NewDefaultTransactionManager
}
