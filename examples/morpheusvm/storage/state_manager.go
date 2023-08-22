// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"github.com/ava-labs/avalanchego/ids"
)

type StateManager struct{}

func (*StateManager) HeightKey() []byte {
	return HeightKey()
}

func (*StateManager) FeeKey() []byte {
	return FeeKey()
}

func (*StateManager) IncomingWarpKeyPrefix(sourceChainID ids.ID, msgID ids.ID) []byte {
	return IncomingWarpKeyPrefix(sourceChainID, msgID)
}

func (*StateManager) OutgoingWarpKeyPrefix(txID ids.ID) []byte {
	return OutgoingWarpKeyPrefix(txID)
}
