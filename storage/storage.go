// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/database/corruptabledb"

	"github.com/ava-labs/hypersdk/pebble"
	"github.com/ava-labs/hypersdk/utils"
)

// TODO: add option to use a single DB with prefixes to allow for atomic writes
func New(chainDataDir string, gatherer metrics.MultiGatherer) (database.Database, database.Database, database.Database, error) {
	// TODO: tune Pebble config based on each sub-db focus
	cfg := pebble.NewDefaultConfig()
	blockPath, err := utils.InitSubDirectory(chainDataDir, block)
	if err != nil {
		return nil, nil, nil, err
	}
	blockDB, blockDBRegistry, err := pebble.New(blockPath, cfg)
	if err != nil {
		return nil, nil, nil, err
	}
	if gatherer != nil {
		if err := gatherer.Register(block, blockDBRegistry); err != nil {
			return nil, nil, nil, err
		}
	}
	statePath, err := utils.InitSubDirectory(chainDataDir, state)
	if err != nil {
		return nil, nil, nil, err
	}
	stateDB, stateDBRegistry, err := pebble.New(statePath, cfg)
	if err != nil {
		return nil, nil, nil, err
	}
	if gatherer != nil {
		if err := gatherer.Register(state, stateDBRegistry); err != nil {
			return nil, nil, nil, err
		}
	}
	metaPath, err := utils.InitSubDirectory(chainDataDir, metadata)
	if err != nil {
		return nil, nil, nil, err
	}
	metaDB, metaDBRegistry, err := pebble.New(metaPath, cfg)
	if err != nil {
		return nil, nil, nil, err
	}
	if gatherer != nil {
		if err := gatherer.Register(metadata, metaDBRegistry); err != nil {
			return nil, nil, nil, err
		}
	}
	return corruptabledb.New(blockDB), corruptabledb.New(stateDB), corruptabledb.New(metaDB), nil
}
