package actions

import (
	"context"
	"encoding/binary"
	"testing"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/tstate"
)

func TestTranferAction(t *testing.T) {
	ts := tstate.New(1)

	tests := map[string]chain.ActionTest{
		"ZeroTransfer": {
			Action: &Transfer{
				To:    codec.EmptyAddress,
				Value: 0,
			},
			ExpectedErr: ErrOutputValueZero,
		},
		"InvalidStateKey": {
			Action: &Transfer{
				To:    codec.EmptyAddress,
				Value: 1,
			},
			State:       ts.NewView(map[string]state.Permissions{}, map[string][]byte{}),
			ExpectedErr: tstate.ErrInvalidKeyOrPermission,
		},
		"NotEnoughBalance": {
			Action: &Transfer{
				To:    codec.EmptyAddress,
				Value: 1,
			},
			Actor: codec.EmptyAddress,
			State: func() state.Mutable {
				keys := make(state.Keys)
				k := storage.BalanceKey(codec.EmptyAddress)
				keys.Add(string(k), state.Read)
				tsv := ts.NewView(keys, map[string][]byte{})
				return tsv
			}(),
			ExpectedErr: storage.ErrInvalidBalance,
		},
		"SimpleTransfer": {
			Action: &Transfer{
				To:    codec.EmptyAddress,
				Value: 1,
			},
			Actor: codec.EmptyAddress,
			State: func() state.Mutable {
				keys := make(state.Keys)
				k := storage.BalanceKey(codec.EmptyAddress)
				keys.Add(string(k), state.All)
				context := context.TODO()
				stor := map[string][]byte{}
				tsv := ts.NewView(keys, stor)
				b := make([]byte, 8)
				binary.LittleEndian.PutUint64(b, uint64(1))
				tsv.Insert(context, k, b)
				return tsv
			}(),
		},
	}

	testSuite := chain.ActionTestSuite{
		Tests: tests,
	}

	testSuite.Run(t)
}
