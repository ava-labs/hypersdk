// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tstate

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/maybe"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/ava-labs/hypersdk/state"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type op struct {
	k string

	pastExists  bool
	pastV       []byte
	pastChanged bool
}

// TState defines a struct for storing temporary state.
type TState struct {
	l           sync.RWMutex
	ops         int
	changedKeys map[string]maybe.Maybe[[]byte]
}

// New returns a new instance of TState. Initializes the storage and changedKeys
// maps to have an initial size of [storageSize] and [changedSize] respectively.
func New(changedSize int) *TState {
	return &TState{
		changedKeys: make(map[string]maybe.Maybe[[]byte], changedSize),
	}
}

func (ts *TState) getChangedValue(_ context.Context, key string) ([]byte, bool, bool) {
	ts.l.RLock()
	defer ts.l.RUnlock()

	if v, ok := ts.changedKeys[key]; ok {
		if v.IsNothing() {
			return nil, true, false
		}
		return v.Value(), true, true
	}
	return nil, false, false
}

func (ts *TState) PendingChanges() int {
	ts.l.RLock()
	defer ts.l.RUnlock()

	return len(ts.changedKeys)
}

// OpIndex returns the number of operations done on ts.
func (ts *TState) OpIndex() int {
	ts.l.RLock()
	defer ts.l.RUnlock()

	return ts.ops
}

// CreateMerkleView creates a slice of [database.BatchOp] of all
// changes in [TState] that can be used to commit to [merkledb].
func (ts *TState) CreateMerkleView(
	ctx context.Context,
	t trace.Tracer, //nolint:interfacer
	view state.View,
) (merkledb.TrieView, error) {
	ts.l.RLock()
	defer ts.l.RUnlock()

	ctx, span := t.Start(
		ctx, "TState.CreateMerkleView",
		oteltrace.WithAttributes(
			attribute.Int("items", len(ts.changedKeys)),
		),
	)
	defer span.End()

	fmt.Println("creating view with changed keys:")
	for k, v := range ts.changedKeys {
		fmt.Println(hex.EncodeToString([]byte(k)), v)
	}

	return view.NewView(ctx, merkledb.ViewChanges{MapOps: ts.changedKeys, ConsumeBytes: true})
}
