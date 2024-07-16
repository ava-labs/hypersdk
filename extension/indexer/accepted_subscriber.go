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
