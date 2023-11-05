// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"

	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/storage"
	"github.com/ava-labs/hypersdk/x/programs/examples/imports/program"
	"github.com/ava-labs/hypersdk/x/programs/examples/imports/pstate"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

var _ chain.Action = (*ProgramExecute)(nil)

type ProgramExecute struct {
	Function string              `json:"programFunction"`
	MaxUnits uint64              `json:"maxUnits"`
	Params   []runtime.CallParam `json:"params"`

	Log logging.Logger

	rt runtime.Runtime
}

func (*ProgramExecute) GetTypeID() uint8 {
	return programExecuteID
}

func (t *ProgramExecute) StateKeys(rauth chain.Auth, id ids.ID) []string {
	return []string{string(storage.ProgramKey(id))}
}

func (*ProgramExecute) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.ProgramChunks}
}

func (*ProgramExecute) OutputsWarpMessage() bool {
	return false
}

func (t *ProgramExecute) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	_ chain.Auth,
	id ids.ID,
	_ bool,
) (bool, uint64, []byte, *warp.UnsignedMessage, error) {
	if len(t.Function) == 0 {
		return false, 1, OutputValueZero, nil, nil
	}
	if len(t.Params) == 0 {
		return false, 1, OutputValueZero, nil, nil
	}

	programIDStr, ok := t.Params[0].Value.(string)
	if !ok {
		return false, 1, utils.ErrBytes(fmt.Errorf("invalid call param: must be ID")), nil, nil
	}

	// TODO: take fee out of balance?
	programID, err := ids.FromString(programIDStr)
	if err != nil {
		return false, 1, utils.ErrBytes(err), nil, nil
	}

	programBytes, err := storage.GetProgram(ctx, mu, programID)
	if err != nil {
		return false, 1, utils.ErrBytes(err), nil, nil
	}

	// TODO: get cfg from genesis
	cfg, err := runtime.NewConfigBuilder().
		WithDebugMode(true).
		WithBulkMemory(true).
		// only required for Wasi support exposed by testing only.
		Build()
	if err != nil {
		return false, 1, utils.ErrBytes(err), nil, nil
	}

	// TODO: allow configurable imports?
	supported := runtime.NewSupportedImports()
	supported.Register("state", func() runtime.Import {
		return pstate.New(logging.NoLog{}, mu)
	})
	supported.Register("program", func() runtime.Import {
		return program.New(logging.NoLog{}, mu, cfg)
	})

	t.rt = runtime.New(logging.NoLog{}, cfg, supported.Imports())
	err = t.rt.Initialize(ctx, programBytes, t.MaxUnits)
	if err != nil {
		return false, 1, utils.ErrBytes(err), nil, nil
	}
	defer t.rt.Stop()

	params, err := runtime.WriteParams(t.rt.Memory(), t.Params)
	if err != nil {
		return false, 1, utils.ErrBytes(err), nil, nil
	}

	resp, err := t.rt.Call(ctx, t.Function, params...)
	if err != nil {
		return false, 1, utils.ErrBytes(err), nil, nil
	}

	// TODO: remove this is to support readonly response for now.
	p := codec.NewWriter(len(resp), consts.MaxInt)
	for _, r := range resp {
		p.PackUint64(r)
	}

	return true, 1, p.Bytes(), nil, nil
}

func (*ProgramExecute) MaxComputeUnits(chain.Rules) uint64 {
	return ProgramExecuteComputeUnits
}

func (*ProgramExecute) Size() int {
	return ed25519.PublicKeyLen + consts.Uint64Len
}

func (t *ProgramExecute) Marshal(p *codec.Packer) {
	// TODO
}

func (t *ProgramExecute) GetBalance() uint64 {
	return t.rt.Meter().GetBalance()
}

func UnmarshalProgramExecute(p *codec.Packer, _ *warp.Message) (chain.Action, error) {
	// TODO
	return nil, nil
}

func (*ProgramExecute) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}
