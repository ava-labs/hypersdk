// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package controller

import (
	"context"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/extensions/indexer"
)

var _ indexer.SuccessfulTxIndexer = &successfulTxIndexer{}

type successfulTxIndexer struct {
	c *Controller
}

func (s *successfulTxIndexer) Accepted(_ context.Context, tx *chain.Transaction, result *chain.Result) error {
	for _, action := range tx.Actions {
		switch action.(type) { //nolint:gocritic
		case *actions.Transfer:
			s.c.metrics.transfer.Inc()
		}
	}
	return nil
}
