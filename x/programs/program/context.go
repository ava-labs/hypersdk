package program

import (
	"github.com/ava-labs/avalanchego/ids"
)

type Context struct {
	ProgramID ids.ID `json:"program"`
	Gas       uint64 `json:"gas_remaining"`
	//Actor     [32]byte `json:"actor"`
}
