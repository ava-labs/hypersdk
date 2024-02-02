package program

import "github.com/ava-labs/avalanchego/ids"

type Context struct {
	ProgramID ids.ID `json:"program"`
}
