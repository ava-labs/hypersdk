// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package controller

import (
	"context"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	"github.com/ava-labs/hypersdk/extensions/indexer"
)

var _ indexer.SuccessfulTxIndexer = &successfulTxIndexer{}

type successfulTxIndexer struct {
	c *Controller
}

func (s *successfulTxIndexer) Accepted(_ context.Context, tx *chain.Transaction, result *chain.Result) error {
	for i, act := range tx.Actions {
		switch action := act.(type) {
		case *actions.CreateAsset:
			s.c.metrics.createAsset.Inc()
		case *actions.MintAsset:
			s.c.metrics.mintAsset.Inc()
		case *actions.BurnAsset:
			s.c.metrics.burnAsset.Inc()
		case *actions.Transfer:
			s.c.metrics.transfer.Inc()
		case *actions.CreateOrder:
			s.c.metrics.createOrder.Inc()
			s.c.orderBook.Add(chain.CreateActionID(tx.ID(), uint8(i)), tx.Auth.Actor(), action)
		case *actions.FillOrder:
			s.c.metrics.fillOrder.Inc()
			outputs := result.Outputs[i]
			for _, output := range outputs {
				orderResult, err := actions.UnmarshalOrderResult(output)
				if err != nil {
					// This should never happen
					return err
				}
				if orderResult.Remaining == 0 {
					s.c.orderBook.Remove(action.Order)
					continue
				}
				s.c.orderBook.UpdateRemaining(action.Order, orderResult.Remaining)
			}
		case *actions.CloseOrder:
			s.c.metrics.closeOrder.Inc()
			s.c.orderBook.Remove(action.Order)
		}
	}

	return nil
}
