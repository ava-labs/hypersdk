package controller

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
)

type StateManager struct{}

func (sm *StateManager) IncomingWarpKey(msgID ids.ID) []byte {
	return storage.IncomingWarpKeyPrefix(msgID)
}

func (sm *StateManager) OutgoingWarpKey(txID ids.ID) []byte {
	return storage.OutgoingWarpKeyPrefix(txID)
}
