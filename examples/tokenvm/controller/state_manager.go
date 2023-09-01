// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package controller

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
)

type StateManager struct{}

func (*StateManager) HeightKey() []byte {
	return storage.HeightKey()
}

func (*StateManager) TimestampKey() []byte {
	return storage.TimestampKey()
}

func (*StateManager) FeeKey() []byte {
	return storage.HeightKey()
}

func (*StateManager) IncomingWarpKeyPrefix(sourceChainID ids.ID, msgID ids.ID) []byte {
	return storage.IncomingWarpKeyPrefix(sourceChainID, msgID)
}

func (*StateManager) OutgoingWarpKeyPrefix(txID ids.ID) []byte {
	return storage.OutgoingWarpKeyPrefix(txID)
}
