// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package controller

import (
	"context"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	"github.com/ava-labs/hypersdk/extension/indexer"
)

var _ indexer.SuccessfulTxSubscriber = (*actionHandler)(nil)

type actionHandler struct {
	c *Controller
}

func (a *actionHandler) Accepted(_ context.Context, tx *chain.Transaction, result *chain.Result) error {
	for i, act := range tx.Actions {
		switch action := act.(type) {
		case *actions.CreateAsset:
			a.c.metrics.createAsset.Inc()
		case *actions.MintAsset:
			a.c.metrics.mintAsset.Inc()
		case *actions.BurnAsset:
			a.c.metrics.burnAsset.Inc()
		case *actions.Transfer:
			a.c.metrics.transfer.Inc()
		case *actions.CreateOrder:
			a.c.metrics.createOrder.Inc()
			a.c.orderBook.Add(chain.CreateActionID(tx.ID(), uint8(i)), tx.Auth.Actor(), action)
		case *actions.FillOrder:
			a.c.metrics.fillOrder.Inc()
			outputs := result.Outputs[i]
			for _, output := range outputs {
				orderResult, err := actions.UnmarshalOrderResult(output)
				if err != nil {
					// This should never happen
					return err
				}
				if orderResult.Remaining == 0 {
					a.c.orderBook.Remove(action.Order)
					continue
				}
				a.c.orderBook.UpdateRemaining(action.Order, orderResult.Remaining)
			}
		case *actions.CloseOrder:
			a.c.metrics.closeOrder.Inc()
			a.c.orderBook.Remove(action.Order)
		}
	}

	return nil
}
