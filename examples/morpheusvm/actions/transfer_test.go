package actions

import (
	"context"
	"testing"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/tstate"
)

func TestTranferAction(t *testing.T) {
	ts := tstate.New(1)

	keys := make(state.Keys)
	one := []byte{0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,1}
	keys.Add(string(one), state.Read)

	tsv := ts.NewView(keys, map[string][]byte{})
	tsv.Insert(context.TODO(), one, []byte{1})

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
			Actor:       codec.EmptyAddress,
			State:       ts.NewView(keys, map[string][]byte{}),
			ContainsErrString: chain.ErrInvalidBalance.Error(),
		},
		"SimpleTransfer": {
			Action: &Transfer{
				To:    codec.EmptyAddress,
				Value: 1,
			},
			Actor:       codec.EmptyAddress,
			State:       tsv,
			ExpectedOutputs: [][]byte{},
		},
	}

	testSuite := chain.ActionTestSuite{
		Tests: tests,
	}

	testSuite.Run(t)
}
