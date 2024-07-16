// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package indexer

import (
	"context"

	"github.com/ava-labs/hypersdk/chain"
)

type AcceptedSubscriber interface {
	Accepted(ctx context.Context, blk *chain.StatelessBlock) error
}

type CombinedAcceptedSubscriber struct {
	subscribers []AcceptedSubscriber
}

func NewCombinedAcceptedSubscriber(subscribers ...AcceptedSubscriber) *CombinedAcceptedSubscriber {
	return &CombinedAcceptedSubscriber{subscribers: subscribers}
}

func (c *CombinedAcceptedSubscriber) Accepted(ctx context.Context, blk *chain.StatelessBlock) error {
	for _, subscriber := range c.subscribers {
		if err := subscriber.Accepted(ctx, blk); err != nil {
			return err
		}
	}
	return nil
}
