// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package throughput

import (
	"context"
	"encoding/binary"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/rand"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/examples/hyperevm/actions"
	"github.com/ava-labs/hypersdk/examples/hyperevm/storage"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"
)

func TestGetTransfer(t *testing.T) {
	r := require.New(t)
	sh := &SpamHelper{
		KeyType: "ed25519",
	}

	// Creating accounts
	to, err := sh.CreateAccount()
	r.NoError(err)
	from, err := sh.CreateAccount()
	r.NoError(err)
	fromFactory, err := auth.GetFactory(from)
	r.NoError(err)

	memo := binary.BigEndian.AppendUint64([]byte{}, rand.Uint64())
	transfers := sh.GetTransfer(
		to.Address,
		1,
		memo,
		fromFactory,
	)
	r.Len(transfers, 1)
	evmCall, ok := transfers[0].(*actions.EvmCall)
	r.True(ok)

	// Initialize sender balance
	mp := make(state.MutableStorage)
	bh := &storage.BalanceHandler{}
	r.NoError(bh.AddBalance(context.Background(), from.Address, mp, 1))

	// Defining tx context
	blockCtx := chain.NewBlockContext(0, time.Now().UnixMilli())
	rules := genesis.NewDefaultRules()
	rules.MaxBlockUnits = fees.Dimensions{1800000, consts.MaxUint64, consts.MaxUint64, consts.MaxUint64, consts.MaxUint64}

	ts := tstate.New(0)
	tsv := ts.NewView(evmCall.Keys, mp, 0)
	output, err := evmCall.Execute(context.Background(), blockCtx, rules, tsv, from.Address, ids.Empty)
	r.NoError(err)

	result, ok := output.(*actions.EvmCallResult)
	r.True(ok)
	r.True(result.Success)
	r.Equal(actions.NilError, result.ErrorCode)
	r.Equal(evmCall.GasLimit, result.UsedGas)
	r.Equal(common.Address{}, result.ContractAddress)
}
