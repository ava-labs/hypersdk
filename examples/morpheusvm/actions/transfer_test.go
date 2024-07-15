package actions

import (
	"testing"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
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
		"SimpleTransfer": {
			Action: &Transfer{
				To:    codec.EmptyAddress,
				Value: 1,
			},
			Actor:       codec.EmptyAddress,
			State:       ts.NewView(map[string]state.Permissions{}, map[string][]byte{}),
			ExpectedErr: tstate.ErrInvalidKeyOrPermission,
		},
	}
	testSuite := chain.ActionTestSuite{
		Tests: tests,
	}
	testSuite.Run(t)
}
