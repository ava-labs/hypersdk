package merkle

import (
	"context"
	"sync"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/maybe"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/ava-labs/hypersdk/smap"
)

const (
	stateInitialSize   = 10_000_000
	pendingInitialSize = 1_000_000
)

type Merkle struct {
	state   map[string][]byte
	mdb     merkledb.MerkleDB
	pending map[string]maybe.Maybe[[]byte]

	l  sync.RWMutex
	cl sync.Mutex
}

func New(db database.Database, vm VM, cfg Config) (*Merkle, error) {
	mdb, err := merkledb.New(context.TODO(), db, merkledb.Config{
		BranchFactor: cfg.GetStateBranchFactor(),
		// RootGenConcurrency limits the number of goroutines
		// that will be used across all concurrent root generations.
		RootGenConcurrency:          cfg.GetRootGenerationCores(),
		HistoryLength:               cfg.GetStateHistoryLength(),
		ValueNodeCacheSize:          cfg.GetValueNodeCacheSize(),
		IntermediateNodeCacheSize:   cfg.GetIntermediateNodeCacheSize(),
		IntermediateWriteBufferSize: cfg.GetStateIntermediateWriteBufferSize(),
		IntermediateWriteBatchSize:  cfg.GetStateIntermediateWriteBatchSize(),
		Reg:                         vm.GetMerkleRegistry(),
		TraceLevel:                  merkledb.InfoTrace,
		Tracer:                      vm.GetTracer(),
	})
	if err != nil {
		return nil, err
	}
	return &Merkle{
		state:   make(map[string][]byte, stateInitialSize),
		mdb:     mdb,
		pending: make(map[string]maybe.Maybe[[]byte], pendingInitialSize),
	}, nil
}

func (m *Merkle) Update(ops *smap.SMap[maybe.Maybe[[]byte]]) {
	m.l.Lock()
	defer m.l.Unlock()

	ops.Iterate(func(key string, value maybe.Maybe[[]byte]) bool {
		m.pending[key] = value
		if value.IsNothing() {
			delete(m.state, key)
		} else {
			m.state[key] = value.Value()
		}
		return true
	})
}

func (m *Merkle) GetValue(key []byte) ([]byte, error) {
	m.l.RLock()
	defer m.l.RUnlock()

	value, ok := m.state[string(key)]
	if !ok {
		return nil, database.ErrNotFound
	}
	return value, nil
}

func (m *Merkle) GetValues(keys [][]byte) ([][]byte, []error) {
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
			values[i] = value
		}
	}
	return values, errors
}

func (m *Merkle) PrepareCommit() func(context.Context) (ids.ID, error) {
	m.l.Lock()
	defer m.l.Unlock()

	pending := m.pending
	m.pending = make(map[string]maybe.Maybe[[]byte], pendingInitialSize)
	return func(ctx context.Context) (ids.ID, error) {
		m.cl.Lock()
		defer m.cl.Unlock()

		// We don't consume bytes because we don't pre-copy them into [pending] (in case
		// they are later replaced).
		view, err := m.mdb.NewView(ctx, merkledb.ViewChanges{MapOps: pending})
		if err != nil {
			return ids.Empty, err
		}
		if err := view.CommitToDB(ctx); err != nil {
			return ids.Empty, err
		}
		return m.mdb.GetMerkleRoot(ctx)
	}
}

func (m *Merkle) Close() error {
	return m.mdb.Close()
}
