package examples

import (
	"context"

	"github.com/ava-labs/avalanchego/database/manager"
	"github.com/ava-labs/avalanchego/trace"
	avago_version "github.com/ava-labs/avalanchego/version"
	"github.com/ava-labs/avalanchego/x/merkledb"

	"github.com/ava-labs/hypersdk/pebble"
	"github.com/ava-labs/hypersdk/state"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	defaultRequestKeyLimit = 2048
)

func newDB(ctx context.Context, dname string) (*state.SimpleMutable, error) {
	pdb, _, err := pebble.New(dname, pebble.NewDefaultConfig())
	if err != nil {
		return nil, err
	}
	manager, err := manager.NewManagerFromDBs([]*manager.VersionedDatabase{
		{
			Database: pdb,
			Version:  avago_version.CurrentDatabase,
		},
	})
	if err != nil {
		return nil, err
	}

	db, err := merkledb.New(ctx, manager.Current().Database, newDefaultDBConfig())
	if err != nil {
		return nil, err
	}

	return state.NewSimpleMutable(db), nil
}

func newDefaultDBConfig() merkledb.Config {
	return merkledb.Config{
		EvictionBatchSize:         100,
		HistoryLength:             defaultRequestKeyLimit,
		ValueNodeCacheSize:        defaultRequestKeyLimit,
		IntermediateNodeCacheSize: defaultRequestKeyLimit,
		Reg:                       prometheus.NewRegistry(),
		Tracer:                    trace.Noop,
	}
}
