// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package controller

import (
	"context"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/programsvm/actions"
	"github.com/ava-labs/hypersdk/extension/indexer"
)

var _ indexer.SuccessfulTxSubscriber = (*actionHandler)(nil)

type actionHandler struct {
	c *Controller
}

func (a *actionHandler) Accepted(_ context.Context, tx *chain.Transaction, _ *chain.Result) error {
	for _, action := range tx.Actions {
		switch action.(type) { //nolint:gocritic
		case *actions.Transfer:
			a.c.metrics.transfer.Inc()
		}
	}
	return nil
}
