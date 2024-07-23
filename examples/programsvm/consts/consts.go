// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package consts

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/runtime"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
)

const (
	HRP      = "morpheus"
	Name     = "programsvm"
	Symbol   = "RED"
	Decimals = 9
)

var ID ids.ID

func init() {
	b := make([]byte, ids.IDLen)
	copy(b, []byte(Name))
	vmID, err := ids.ToID(b)
	if err != nil {
		panic(err)
	}
	ID = vmID
}

// Instantiate registry here so it can be imported by any package. We set these
// values in [controller/registry].
var (
	ActionRegistry *codec.TypeParser[chain.Action]
	AuthRegistry   *codec.TypeParser[chain.Auth]

	// shameless hacks to make program call action have access to the runtime and db for execution and statekey queries

	ProgramRuntime *runtime.WasmRuntime
	StateKeysDB    state.Immutable
)
