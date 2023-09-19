// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"context"

	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/tokenvm/cmd/token-feed/manager"
)

type Manager interface {
	GetFeedInfo(context.Context) (ed25519.PublicKey, uint64, error)
	GetFeed(context.Context) ([]*manager.FeedObject, error)
}
