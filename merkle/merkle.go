package merkle

import (
	"context"
	"slices"
	"sync"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/maybe"
	"github.com/ava-labs/avalanchego/x/merkledb"
	ssync "github.com/ava-labs/avalanchego/x/sync"
	"github.com/ava-labs/hypersdk/smap"
	"github.com/ava-labs/hypersdk/state"
)

var (
	_ state.Immutable = (*Merkle)(nil)
	_ state.Mutable   = (*Merkle)(nil)
	_ state.Database  = (*Merkle)(nil)

	_ ssync.DB = (*Merkle)(nil)
)

const (
	stateInitialSize   = 10_000_000
	pendingInitialSize = 1_000_000
)

type change struct {
	key string
	val maybe.Maybe[[]byte]
}

type Merkle struct {
	l     sync.RWMutex
	size  uint64
	state map[string][]byte

	cl  sync.Mutex
	mdb merkledb.MerkleDB

	rv          *merkledb.RollingView
	pending     chan *change
	pendingDone chan struct{}
}

func New(ctx context.Context, db database.Database, cfg merkledb.Config) (*Merkle, error) {
	mdb, err := merkledb.New(ctx, db, cfg)
	if err != nil {
		return nil, err
	}
	rv, err := mdb.NewRollingView(ctx)
	if err != nil {
		return nil, err
	}
	pending := make(chan *change, pendingInitialSize)
	pendingDone := make(chan struct{})
	go func() {
		defer close(pendingDone)
		for c := range pending {
			if err := rv.Process(context.Background(), c.key, c.val); err != nil {
				panic(err)
			}
		}
	}()
	// TODO: load values into [state]
	return &Merkle{
		state:       make(map[string][]byte, stateInitialSize),
		mdb:         mdb,
		rv:          rv,
		pending:     pending,
		pendingDone: pendingDone,
	}, nil
}

// TODO: use smap for merkle and update shards concurrently
func (m *Merkle) Update(_ context.Context, ops *smap.SMap[maybe.Maybe[[]byte]]) int {
	m.l.Lock()
	defer m.l.Unlock()

	seen := 0
	ops.Iterate(func(key string, value maybe.Maybe[[]byte]) bool {
		seen++
		m.pending <- &change{key: key, val: value}
		if value.IsNothing() {
			m.stateRemove(key)
		} else {
			m.stateInsert(key, value.Value())
		}
		return true
	})
	return seen
}

func (m *Merkle) Usage() (int, uint64) {
	m.l.RLock()
	defer m.l.RUnlock()

	return len(m.state), m.size
}

func (m *Merkle) PrepareCommit(context.Context) (func(context.Context) (ids.ID, error), int) {
	m.l.Lock()
	defer m.l.Unlock()

	// Create new channels (ensure we don't update goroutine)
	pending := m.pending
	pendingDone := m.pendingDone
	newPending := make(chan *change, pendingInitialSize)
	m.pending = newPending
	newPendingDone := make(chan struct{})
	m.pendingDone = newPendingDone

	m.cl.Lock()
	return func(ctx context.Context) (ids.ID, error) {
		defer m.cl.Unlock()

		// Wait for processing to finish
		close(pending)
		<-pendingDone

		// Create new rv
		oldRv := m.rv
		newRv, err := m.rv.NewRollingView(ctx)
		if err != nil {
			return ids.Empty, err
		}
		m.rv = newRv

		// Start new processing queue
		go func() {
			defer close(newPendingDone)
			for c := range newPending {
				if err := newRv.Process(context.Background(), c.key, c.val); err != nil {
					panic(err)
				}
			}
		}()

		// We don't consume bytes because we don't pre-copy them into [pending] (in case
		// they are later replaced).
		if err := oldRv.CommitToDB(ctx); err != nil {
			return ids.Empty, err
		}
		return m.mdb.GetMerkleRoot(ctx)
	}, len(pending)
}

func (m *Merkle) stateInsert(key string, value []byte) {
	past, ok := m.state[key]
	if ok {
		m.size -= uint64(len(past))
	} else {
		m.size += uint64(len(key))
	}
	m.size += uint64(len(value))
	m.state[key] = value
}

func (m *Merkle) stateRemove(key string) {
	past, ok := m.state[key]
	if ok {
		m.size -= uint64(len(key) + len(past))
	}
	delete(m.state, key)
}

// We assume that any bytes provided to Insert can be consumed.
func (m *Merkle) Insert(_ context.Context, key, value []byte) error {
	m.l.Lock()
	defer m.l.Unlock()

	m.pending <- &change{key: string(key), val: maybe.Some(value)}
	m.stateInsert(string(key), value)
	return nil
}

func (m *Merkle) Remove(_ context.Context, key []byte) error {
	m.l.Lock()
	defer m.l.Unlock()

	m.pending <- &change{key: string(key), val: maybe.Nothing[[]byte]()}
	m.stateRemove(string(key))
	return nil
}

// Any bytes returned by [GetValue] can be modified.
func (m *Merkle) GetValue(_ context.Context, key []byte) ([]byte, error) {
	m.l.RLock()
	defer m.l.RUnlock()

	value, ok := m.state[string(key)]
	if !ok {
		return nil, database.ErrNotFound
	}
	return slices.Clone(value), nil
}

// Any bytes returned by [GetValues] can be modified.
func (m *Merkle) GetValues(_ context.Context, keys [][]byte) ([][]byte, []error) {
	m.l.RLock()
	defer m.l.RUnlock()

	var (
		values = make([][]byte, len(keys))
		errors = make([]error, len(keys))
	)
	for i, key := range keys {
		value, ok := m.state[string(key)]
		if !ok {
			errors[i] = database.ErrNotFound
		} else {
			values[i] = slices.Clone(value)
		}
	}
	return values, errors
}

// Implement [sync.DB] interface
func (m *Merkle) Clear() error {
	// TODO: may not be correct (shouldn't be adding keys anywhere)
	return m.mdb.Clear()
}

func (m *Merkle) GetMerkleRoot(_ context.Context) (ids.ID, error) {
	return m.mdb.GetMerkleRoot(context.Background())
}

func (m *Merkle) GetProof(ctx context.Context, keyBytes []byte) (*merkledb.Proof, error) {
	return m.mdb.GetProof(ctx, keyBytes)
}

func (m *Merkle) GetChangeProof(
	ctx context.Context,
	startRootID ids.ID,
	endRootID ids.ID,
	start maybe.Maybe[[]byte],
	end maybe.Maybe[[]byte],
	maxLength int,
) (*merkledb.ChangeProof, error) {
	return m.mdb.GetChangeProof(ctx, startRootID, endRootID, start, end, maxLength)
}

func (m *Merkle) VerifyChangeProof(
	ctx context.Context,
	proof *merkledb.ChangeProof,
	start maybe.Maybe[[]byte],
	end maybe.Maybe[[]byte],
	expectedEndRootID ids.ID,
) error {
	return m.mdb.VerifyChangeProof(ctx, proof, start, end, expectedEndRootID)
}

func (m *Merkle) GetRangeProofAtRoot(
	ctx context.Context,
	rootID ids.ID,
	start maybe.Maybe[[]byte],
	end maybe.Maybe[[]byte],
	maxLength int,
) (*merkledb.RangeProof, error) {
	return m.mdb.GetRangeProofAtRoot(ctx, rootID, start, end, maxLength)
}

func (m *Merkle) CommitRangeProof(ctx context.Context, start, end maybe.Maybe[[]byte], proof *merkledb.RangeProof) error {
	return m.mdb.CommitRangeProof(ctx, start, end, proof)
}

func (m *Merkle) CommitChangeProof(ctx context.Context, proof *merkledb.ChangeProof) error {
	return m.mdb.CommitChangeProof(ctx, proof)
}

func (m *Merkle) Close() error {
	return m.mdb.Close()
}
