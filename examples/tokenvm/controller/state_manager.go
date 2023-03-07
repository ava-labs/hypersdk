// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package controller

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
)

type StateManager struct{}

func (*StateManager) IncomingWarpKey(msgID ids.ID) []byte {
	return storage.IncomingWarpKeyPrefix(msgID)
}

func (*StateManager) OutgoingWarpKey(txID ids.ID) []byte {
	return storage.OutgoingWarpKeyPrefix(txID)
}
