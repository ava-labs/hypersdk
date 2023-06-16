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

func (*StateManager) ParentUnitWindowKey() []byte {
	return storage.ParentUnitWindowKey()
}
func (*StateManager) ParentUnitsConsumedKey() []byte {
	return storage.ParentUnitsConsumedKey()
}
func (*StateManager) ParentUnitPriceKey() []byte {
	return storage.ParentUnitPriceKey()
}

func (*StateManager) IncomingWarpKey(sourceChainID ids.ID, msgID ids.ID) []byte {
	return storage.IncomingWarpKeyPrefix(sourceChainID, msgID)
}

func (*StateManager) OutgoingWarpKey(txID ids.ID) []byte {
	return storage.OutgoingWarpKeyPrefix(txID)
}
