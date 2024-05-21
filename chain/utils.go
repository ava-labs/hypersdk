package chain

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/utils"
)

func CreateActionID(txID ids.ID, i uint8) ids.ID {
	actionBytes := make([]byte, ids.IDLen+consts.Uint8Len)
	copy(actionBytes, txID[:])
	actionBytes[ids.IDLen] = i
	return utils.ToID(actionBytes)
}
