/*
 * Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
 * See the file LICENSE for licensing terms.
 */

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package indexer

import (
	"context"

	"github.com/ava-labs/hypersdk/chain"
)

var _ AcceptedSubscriber = (*successfulTxSubscriber)(nil)

type SuccessfulTxSubscriber interface {
	Accepted(ctx context.Context, tx *chain.Transaction, result *chain.Result) error
}

// SuccessfulTxSubscriber calls the provided SucccessfulTxSubscriber for each successful transaction
type successfulTxSubscriber struct {
	successfulTxSubscriber SuccessfulTxSubscriber
}

func NewSuccessfulTxSubscriber(subscriber SuccessfulTxSubscriber) AcceptedSubscriber {
	return &successfulTxSubscriber{subscriber}
}

func (s *successfulTxSubscriber) Accepted(ctx context.Context, blk *chain.StatelessBlock) error {
	results := blk.Results()
	for i, tx := range blk.Txs {
		result := results[i]
		if result.Success {
			if err := s.successfulTxSubscriber.Accepted(ctx, tx, result); err != nil {
				return err
			}
		}
	}

	return nil
}
