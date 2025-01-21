// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package defaultvm

import (
	"github.com/ava-labs/hypersdk/api/indexer"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/extension/externalsubscriber"
	"github.com/ava-labs/hypersdk/vm"

	staterpc "github.com/ava-labs/hypersdk/api/state"
)

// DefaultOptions provides the default set of options to include
// when constructing a new VM including the indexer, websocket,
// JSONRPC, and external subscriber options.
func NewDefaultOptions() []vm.Option {
	return []vm.Option{
		indexer.With(),
		ws.With(),
		jsonrpc.With(),
		externalsubscriber.With(),
		staterpc.With(),
	}
}
