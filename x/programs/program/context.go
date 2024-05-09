// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package program

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/near/borsh-go"
)

type Context struct {
	ProgramID ids.ID `json:"program"`
	result    []byte
	// Actor            [32]byte `json:"actor"`
	// OriginatingActor [32]byte `json:"originating_actor"`
}

func (c *Context) Result() []byte {
	return c.result
}

func (c *Context) SetResult(result []byte) {
	c.result = result
}

func (c *Context) ClearResult() {
	c.result = nil
}

type serializeable struct {
	ProgramID ids.ID
}

func (c *Context) WriteToMem(mem *Memory) (uint32, error) {
	bytes, err := borsh.Serialize(serializeable{ProgramID: c.ProgramID})
	if err != nil {
		return 0, err
	}

	return AllocateBytes(bytes, mem)
}
