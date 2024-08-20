// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package indexer

import (
	"context"

	"github.com/ava-labs/hypersdk/chain"
)

type AcceptedSubscriber interface {
	Accepted(ctx context.Context, blk *chain.StatefulBlock, results []*chain.Result) error
}

type AcceptedSubscribers struct {
	subscribers []AcceptedSubscriber
}

func NewAcceptedSubscribers(subscribers ...AcceptedSubscriber) *AcceptedSubscribers {
	return &AcceptedSubscribers{subscribers: subscribers}
}

func (a *AcceptedSubscribers) Accepted(ctx context.Context, blk *chain.StatefulBlock, results []*chain.Result) error {
	for _, subscriber := range a.subscribers {
		if err := subscriber.Accepted(ctx, blk, results); err != nil {
			return err
		}
	}
	return nil
}
