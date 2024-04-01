package merkle

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/maybe"
	"github.com/ava-labs/avalanchego/x/merkledb"
	ssync "github.com/ava-labs/avalanchego/x/sync"
	"github.com/ava-labs/hypersdk/rchannel"
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
	stateInitialSize = 10_000_000

	// backlogInitialSize sets the backlog for trie modification processing
	//
	// 200k keys per block * 60 blocks per minute (max keys ~12M)
	backlogInitialSize = 15_000_000
	changesInitialSize = 2_500_000
)

type Merkle struct {
	l     sync.RWMutex
	size  uint64
	state map[string][]byte

	cl  sync.Mutex
	mdb merkledb.MerkleDB

	rv *merkledb.RollingView
	rc *rchannel.RChannel[maybe.Maybe[[]byte]]
}

func New(ctx context.Context, db database.Database, cfg merkledb.Config) (*Merkle, error) {
	mdb, err := merkledb.New(ctx, db, cfg)
	if err != nil {
		return nil, err
	}
	m := &Merkle{
		// TODO: load state from disk
		state: make(map[string][]byte, stateInitialSize),
		mdb:   mdb,
	}
	m.rv, err = mdb.NewRollingView(ctx, changesInitialSize)
	if err != nil {
		return nil, err
	}
	m.rc = rchannel.New[maybe.Maybe[[]byte]](backlogInitialSize)
	m.rc.SetCallback(m.rv.Update)
	return m, nil
}

// TODO: use smap for merkle and update shards concurrently
func (m *Merkle) Update(_ context.Context, ops *smap.SMap[maybe.Maybe[[]byte]]) int {
	m.l.Lock()
	defer m.l.Unlock()

	seen := 0
	ops.Iterate(func(key string, value maybe.Maybe[[]byte]) bool {
		seen++
		m.rc.Add(key, value)
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

func (m *Merkle) PrepareCommit(context.Context) func(context.Context, state.Metrics) (ids.ID, error) {
	m.l.Lock()
	defer m.l.Unlock()

	// It is safe to add to [rchannel] before the callback is set (pending
	// requests will just be queued on the channel).
	rc := m.rc
	m.rc = rchannel.New[maybe.Maybe[[]byte]](backlogInitialSize)

	m.cl.Lock()
	return func(ctx context.Context, metrics state.Metrics) (ids.ID, error) {
		defer m.cl.Unlock()

		// Wait for trie modifications to finish
		t := time.Now()
		skips, maxBacklog, err := rc.Wait()
		if err != nil {
			return ids.Empty, err
		}
		metrics.RecordWaitTrieModifications(time.Since(t))
		metrics.RecordTrieMaxBacklog(maxBacklog)

		// Merklize trie
		t = time.Now()
		root, err := m.rv.Merklize(context.TODO())
		if err != nil {
			return ids.Empty, err
		}
		metrics.RecordWaitTrieRoot(time.Since(t))
		nodes, values := m.rv.Changes()
		metrics.RecordTrieNodeChanges(nodes)
		metrics.RecordTrieValueChanges(values)
		metrics.RecordTrieSkippedValueChanges(skips)

		// Create new rv once trie is merklized
		oldRv := m.rv
		m.rv, err = m.rv.NewRollingView(ctx, changesInitialSize)
		if err != nil {
			return ids.Empty, err
		}
		m.rc.SetCallback(m.rv.Update)

		// Wait for trie to be committed to disk
		t = time.Now()
		if err := oldRv.Commit(ctx); err != nil {
			return ids.Empty, err
		}
		metrics.RecordWaitTrieCommit(time.Since(t))

		// We wait to return root until the trie
		// is committed to disk.
		return root, nil
	}
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

	skey := string(key)
	m.rc.Add(skey, maybe.Some(value))
	m.stateInsert(skey, value)
	return nil
}

func (m *Merkle) Remove(_ context.Context, key []byte) error {
	m.l.Lock()
	defer m.l.Unlock()

	skey := string(key)
	m.rc.Add(skey, maybe.Nothing[[]byte]())
	m.stateRemove(skey)
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
