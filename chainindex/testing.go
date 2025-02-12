// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chainindex

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/prometheus/client_golang/prometheus"
)

// TestFactory provides easy ways to create test chain indexes
type TestFactory[T Block] struct {
	Config Config
	DB     database.Database
	Parser Parser[T]
}

func NewChainIndexTestFactory[T Block](parser Parser[T]) *TestFactory[T] {
	return &TestFactory[T]{
		Config: NewDefaultConfig(),
		DB:     memdb.New(),
		Parser: parser,
	}
}

func (f *TestFactory[T]) WithConfig(config Config) *TestFactory[T] {
	f.Config = config
	return f
}

func (f *TestFactory[T]) WithDB(db database.Database) *TestFactory[T] {
	f.DB = db
	return f
}

func (f *TestFactory[T]) BuildWithBlocks(ctx context.Context, blocks []T) (*ChainIndex[T], error) {
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

func (f *TestFactory[T]) Build() (*ChainIndex[T], error) {
	if f.DB == nil {
		return nil, errors.New("db not initialized")
	}

	return New[T](logging.NoLog{}, prometheus.NewRegistry(), f.Config, f.Parser, f.DB)
}
