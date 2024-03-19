// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/internal/storage"
)

type ProgramCreate struct {
	Program []byte `json:"program"`
}

func (t *ProgramCreate) Execute(
	ctx context.Context,
	mu state.Mutable,
	id ids.ID,
) (bool, uint64, []byte, *warp.UnsignedMessage, error) {
	if len(t.Program) == 0 {
		return false, 1, OutputValueZero, nil, nil
	}

	if err := storage.SetProgram(ctx, mu, id, t.Program); err != nil {
		return false, 1, utils.ErrBytes(err), nil, nil
	}

	return true, 1, nil, nil, nil
}
