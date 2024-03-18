package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/internal/storage"
)

type ProgramCreate struct {
	Program []byte `json:"program"`
}

func (t *ProgramCreate) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	_ codec.Address,
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
