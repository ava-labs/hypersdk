// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package ws

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/vm"
)

// stubVM implements only the methods the tx message path touches. Any other
// call panics on the embedded nil interface, which keeps the stub honest.
type stubVM struct {
	api.VM

	ruleFactory chain.RuleFactory
	submitErr   error
	submitted   int
}

func (*stubVM) Tracer() trace.Tracer { return trace.Noop }

func (s *stubVM) GetRuleFactory() chain.RuleFactory { return s.ruleFactory }

func (s *stubVM) Submit(context.Context, []*chain.Transaction) []error {
	s.submitted++
	return []error{s.submitErr}
}

func newTestWebSocketServer(t *testing.T, submitErr error) (*WebSocketServer, *stubVM) {
	t.Helper()

	rules := genesis.NewDefaultRules()
	v := &stubVM{
		ruleFactory: &genesis.ImmutableRuleFactory{Rules: rules},
		submitErr:   submitErr,
	}
	// a tiny pending-message queue: this test never publishes, and the default
	// would allocate hundreds of MiB per connection
	w, _ := NewWebSocketServer(v, logging.NoLog{}, trace.Noop, chaintest.NewTestParser(), 8)
	return w, v
}

func newTestTx(t *testing.T, timestamp int64) *chain.Transaction {
	t.Helper()

	tx, err := chain.NewTransaction(
		chain.Base{Timestamp: timestamp},
		[]chain.Action{chaintest.NewDummyTestAction()},
		chaintest.NewDummyTestAuth(),
	)
	require.NoError(t, err)
	return tx
}

// encodeTxMessage frames a tx the way api/ws clients do.
func encodeTxMessage(tx *chain.Transaction) []byte {
	return append([]byte{TxMode}, tx.Bytes()...)
}

func (w *WebSocketServer) listenerCount() int {
	w.txL.Lock()
	defer w.txL.Unlock()

	return len(w.txListeners)
}

// A tx that fails Submit should release its listener immediately rather than
// holding it until its expiry passes.
func TestMessageCallbackRemovesListenerWhenSubmitFails(t *testing.T) {
	r := require.New(t)

	w, v := newTestWebSocketServer(t, errors.New("invalid signature"))
	callback := w.MessageCallback()

	// within the validity window, so it reaches Submit
	tx := newTestTx(t, time.Now().UnixMilli()+1_000)
	callback(encodeTxMessage(tx), nil)

	r.Equal(1, v.submitted, "tx should have reached Submit")
	r.Zero(w.listenerCount(), "listener must be removed after Submit fails")
}

// ErrNotAdded means the tx is already pending in the mempool, so the listener
// has to stay or the client would never be told the outcome.
func TestMessageCallbackKeepsListenerWhenAlreadyInMempool(t *testing.T) {
	r := require.New(t)

	w, v := newTestWebSocketServer(t, vm.ErrNotAdded)
	callback := w.MessageCallback()

	tx := newTestTx(t, time.Now().UnixMilli()+1_000)
	callback(encodeTxMessage(tx), nil)

	r.Equal(1, v.submitted)
	r.Equal(1, w.listenerCount(), "listener must be retained for a tx already in the mempool")
}

// A tx whose expiry is past the validity window can never be executed and can
// never be evicted by setMinTx, so it must be rejected before it is recorded.
// This is the check that actually bounds txListeners: without it a listener is
// retained for the lifetime of the process.
func TestMessageCallbackRejectsExpiryBeyondValidityWindow(t *testing.T) {
	r := require.New(t)

	w, v := newTestWebSocketServer(t, nil)
	callback := w.MessageCallback()

	rules := genesis.NewDefaultRules()
	beyond := time.Now().UnixMilli() + rules.GetValidityWindow() + 10_000
	tx := newTestTx(t, beyond)
	callback(encodeTxMessage(tx), nil)

	r.Zero(v.submitted, "tx beyond the validity window should not reach Submit")
	r.Zero(w.listenerCount(), "no listener should be recorded")

	// Confirm the eviction path really could not have cleared it: advancing the
	// expiry horizon to now leaves a far-future entry in place.
	w.setMinTx(time.Now().UnixMilli())
	r.Zero(w.listenerCount())
}

// A successfully submitted tx keeps its listener, and it is evictable because
// its expiry is inside the validity window.
func TestMessageCallbackListenerIsEvictable(t *testing.T) {
	r := require.New(t)

	w, _ := newTestWebSocketServer(t, nil)
	callback := w.MessageCallback()

	expiry := time.Now().UnixMilli() + 1_000
	tx := newTestTx(t, expiry)
	callback(encodeTxMessage(tx), nil)
	r.Equal(1, w.listenerCount())

	// once the chain advances past the tx expiry, the listener is cleared
	w.setMinTx(expiry + 1)
	r.Zero(w.listenerCount(), "listener should be evicted once its expiry passes")
}

func TestRemoveTxListener(t *testing.T) {
	r := require.New(t)

	w, _ := newTestWebSocketServer(t, nil)
	tx := newTestTx(t, time.Now().UnixMilli()+1_000)

	w.AddTxListener(tx, nil)
	r.Equal(1, w.listenerCount())

	w.RemoveTxListener(tx.GetID(), nil)
	r.Zero(w.listenerCount())

	// removing an unknown txID is a no-op
	w.RemoveTxListener(tx.GetID(), nil)
	r.Zero(w.listenerCount())
}
