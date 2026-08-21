// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package jsonrpc

import (
	"context"
	"net/http"
	"testing"

	"github.com/ava-labs/avalanchego/trace"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/state"
)

// stubVM implements only the methods the simulation endpoints touch. Any other
// call panics on the embedded nil interface, which keeps the stub honest.
type stubVM struct {
	api.VM

	ruleFactory chain.RuleFactory
	parsed      int
}

func (*stubVM) Tracer() trace.Tracer { return trace.Noop }

func (s *stubVM) GetRuleFactory() chain.RuleFactory { return s.ruleFactory }

func (s *stubVM) GetParser() chain.Parser {
	s.parsed++
	return chaintest.NewTestParser()
}

func (*stubVM) ImmutableState(context.Context) (state.Immutable, error) {
	return state.ImmutableStorage(map[string][]byte{}), nil
}

func (*stubVM) ReadState(_ context.Context, keys [][]byte) ([][]byte, []error) {
	return make([][]byte, len(keys)), make([]error, len(keys))
}

func newTestServer(rules chain.Rules) (*JSONRPCServer, *stubVM) {
	v := &stubVM{ruleFactory: &genesis.ImmutableRuleFactory{Rules: rules}}
	return NewJSONRPCServer(v), v
}

func dummyActionBytes(t *testing.T) []byte {
	t.Helper()

	return chaintest.NewDummyTestAction().Bytes()
}

// SimulateActions is unauthenticated and unmetered, so it must bound the action
// count the same way ExecuteActions does. Without this, one request could ask
// the node to parse and execute an arbitrarily long action list against live
// state.
func TestSimulateActionsBoundsActionCount(t *testing.T) {
	rules := genesis.NewDefaultRules()
	maxActions := int(rules.GetMaxActionsPerTx())

	t.Run("over the limit is rejected before parsing", func(t *testing.T) {
		r := require.New(t)

		server, v := newTestServer(rules)

		actionBytes := dummyActionBytes(t)
		actions := make([]codec.Bytes, maxActions+1)
		for i := range actions {
			actions[i] = actionBytes
		}

		err := server.SimulateActions(
			&http.Request{},
			&SimulatActionsArgs{Actions: actions},
			&SimulateActionsReply{},
		)
		r.ErrorContains(err, "exceeded max actions per simulation")
		r.Zero(v.parsed, "the request should be rejected before any action is parsed")
	})

	t.Run("zero actions is rejected", func(t *testing.T) {
		r := require.New(t)

		server, _ := newTestServer(rules)

		err := server.SimulateActions(
			&http.Request{},
			&SimulatActionsArgs{},
			&SimulateActionsReply{},
		)
		r.ErrorIs(err, errSimulateZeroActions)
	})

	t.Run("at the limit is accepted", func(t *testing.T) {
		r := require.New(t)

		server, _ := newTestServer(rules)

		actionBytes := dummyActionBytes(t)
		actions := make([]codec.Bytes, maxActions)
		for i := range actions {
			actions[i] = actionBytes
		}

		reply := &SimulateActionsReply{}
		err := server.SimulateActions(
			&http.Request{},
			&SimulatActionsArgs{Actions: actions},
			reply,
		)
		r.NoError(err)
		r.Len(reply.ActionResults, maxActions)
	})
}

// ExecuteActions already had this bound; keep it covered so the two endpoints
// cannot drift apart again.
func TestExecuteActionsBoundsActionCount(t *testing.T) {
	r := require.New(t)

	rules := genesis.NewDefaultRules()
	server, _ := newTestServer(rules)

	actionBytes := dummyActionBytes(t)
	actions := make([][]byte, int(rules.GetMaxActionsPerTx())+1)
	for i := range actions {
		actions[i] = actionBytes
	}

	err := server.ExecuteActions(
		&http.Request{},
		&ExecuteActionArgs{Actions: actions},
		&ExecuteActionReply{},
	)
	r.ErrorContains(err, "exceeded max actions per simulation")
}
