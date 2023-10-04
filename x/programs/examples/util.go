package examples

import (
	"context"

	"github.com/ava-labs/avalanchego/database/manager"
	"github.com/ava-labs/avalanchego/trace"
	avago_version "github.com/ava-labs/avalanchego/version"
	"github.com/ava-labs/avalanchego/x/merkledb"

	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/pebble"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/runtime"

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

func newKeyPtr(ctx context.Context, key ed25519.PublicKey, runtime runtime.Runtime) (uint64, error) {
	ptr, err := runtime.Memory().Alloc(ed25519.PublicKeyLen)
	if err != nil {
		return 0, err
	}

	// write programID to memory which we will later pass to the program
	err = runtime.Memory().Write(ptr, key[:])
	if err != nil {
		return 0, err
	}

	return ptr, err
}

func newKey() (ed25519.PrivateKey, ed25519.PublicKey, error) {
	priv, err := ed25519.GeneratePrivateKey()
	if err != nil {
		return ed25519.EmptyPrivateKey, ed25519.EmptyPublicKey, err
	}

	return priv, priv.PublicKey(), nil
}
