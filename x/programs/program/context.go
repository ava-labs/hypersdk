package program

import (
	"github.com/ava-labs/avalanchego/ids"
)

type Context struct {
	ProgramID ids.ID `json:"program"`
	// Actor            [32]byte `json:"actor"`
	// OriginatingActor [32]byte `json:"originating_actor"`
}
