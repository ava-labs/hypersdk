// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"

	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/storage"
)

var _ chain.Action = (*ProgramCreate)(nil)

type ProgramCreate struct {
	Program []byte `json:"program"`
}

func (*ProgramCreate) GetTypeID() uint8 {
	return programCreateID
}

func (t *ProgramCreate) StateKeys(rauth chain.Auth, _ ids.ID) []string {
	return []string{}
}

func (*ProgramCreate) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.ProgramChunks}
}

func (*ProgramCreate) OutputsWarpMessage() bool {
	return false
}

func (t *ProgramCreate) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	_ chain.Auth,
	id ids.ID,
	_ bool,
) (bool, uint64, []byte, *warp.UnsignedMessage, error) {
	if len(t.Program) == 0 {
		return false, 1, OutputValueZero, nil, nil
	}

	if err := storage.SetProgram(ctx, mu, id, t.Program); err != nil {
		return false, 1, utils.ErrBytes(err), nil, nil
	}

	return true, 1, nil, nil, nil
}

func (*ProgramCreate) MaxComputeUnits(chain.Rules) uint64 {
	return ProgramCreateComputeUnits
}

func (*ProgramCreate) Size() int {
	return ed25519.PublicKeyLen + consts.Uint64Len
}

func (t *ProgramCreate) Marshal(p *codec.Packer) {
	p.PackBytes(t.Program)
}

func UnmarshalProgramCreate(p *codec.Packer, _ *warp.Message) (chain.Action, error) {
	var pc ProgramCreate
	p.UnpackBytes(-1, true, &pc.Program)
	return &pc, p.Err()
}

func (*ProgramCreate) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}
