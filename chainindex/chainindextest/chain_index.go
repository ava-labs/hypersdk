package chainindextest

import (
	"context"
	"errors"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/chainindex"
	"github.com/prometheus/client_golang/prometheus"
)

// TestFactory provides easy ways to create test chain indexes
type TestFactory[T chainindex.Block] struct {
	Config chainindex.Config
	DB     database.Database
	Parser chainindex.Parser[T]
}

func NewChainIndexTestFactory[T chainindex.Block](parser chainindex.Parser[T]) *TestFactory[T] {
	return &TestFactory[T]{
		Config: chainindex.NewDefaultConfig(),
		DB:     memdb.New(),
		Parser: parser,
	}
}

func (f *TestFactory[T]) WithConfig(config chainindex.Config) *TestFactory[T] {
	f.Config = config
	return f
}

func (f *TestFactory[T]) WithDB(db database.Database) *TestFactory[T] {
	f.DB = db
	return f
}

func (f *TestFactory[T]) BuildWithBlocks(ctx context.Context, blocks []T) (*chainindex.ChainIndex[T], error) {
	ci, err := f.Build()
	if err != nil {
		return nil, err
	}

	for _, blk := range blocks {
		if err := ci.UpdateLastAccepted(ctx, blk); err != nil {
			return nil, err
		}
	}

	return ci, nil
}

func (f *TestFactory[T]) Build() (*chainindex.ChainIndex[T], error) {
	if f.DB == nil {
		return nil, errors.New("db not initialized")
	}

	return chainindex.New[T](logging.NoLog{}, prometheus.NewRegistry(), f.Config, f.Parser, f.DB)
}
