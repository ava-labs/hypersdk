// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package program

import (
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/bytecodealliance/wasmtime-go/v14"

	"github.com/ava-labs/hypersdk/state"

	"github.com/ava-labs/hypersdk/x/programs/engine"
	"github.com/ava-labs/hypersdk/x/programs/host"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

var _ host.Import = (*Import)(nil)

const Name = "reentrancy"

type Import struct {
	mu  state.Mutable
	log logging.Logger
	cfg *runtime.Config

	engine  *engine.Engine
	meter   *engine.Meter
	imports host.SupportedImports

	
	reentrancy *engine.ReentrancyGaurd
}

// New returns a new program invoke host module which can perform program to program calls.
func New(log logging.Logger, engine *engine.Engine, mu state.Mutable, cfg *runtime.Config) *Import {
	return &Import{
		cfg:    cfg,
		mu:     mu,
		log:    log,
		engine: engine,
	}
}

func (i *Import) Name() string {
	return Name
}

func (i *Import) Register(link *host.Link) error {
	i.meter = link.Meter()
	i.imports = link.Imports()
	return link.RegisterImportFn(Name, "with_reentrancy", i.setReentrancy)
}

// callProgramFn makes a call to an entry function of a program in the context of another program's ID.
func (i *Import) setReentrancy(
	wasmCaller *wasmtime.Caller,
	programID int64,
	functionName int64, 
	maxReenters int64,
) int64 {
	// check if the current gaurd if re-entrancy has already been set

	// if not, set the re-entrancy gaurd
	return 0
}
