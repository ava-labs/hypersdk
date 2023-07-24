// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/hypersdk/pebble"
	"github.com/ava-labs/hypersdk/utils"
)

// TODO: add option to use a single DB with prefixes to allow for atomic writes
// TODO: add metrics to pebble
func New(chainDataDir string) (database.Database, database.Database, database.Database, error) {
	// TODO: tune Pebble config based on each sub-db focus
	cfg := pebble.NewDefaultConfig()
	blockPath, err := utils.InitSubDirectory(chainDataDir, blockDir)
	if err != nil {
		return nil, nil, nil, err
	}
	blockDB, err := pebble.New(blockPath, cfg)
	if err != nil {
		return nil, nil, nil, err
	}
	statePath, err := utils.InitSubDirectory(chainDataDir, stateDir)
	if err != nil {
		return nil, nil, nil, err
	}
	stateDB, err := pebble.New(statePath, cfg)
	if err != nil {
		return nil, nil, nil, err
	}
	metaPath, err := utils.InitSubDirectory(chainDataDir, metadataDir)
	if err != nil {
		return nil, nil, nil, err
	}
	metaDB, err := pebble.New(metaPath, cfg)
	if err != nil {
		return nil, nil, nil, err
	}
	return blockDB, stateDB, metaDB, nil
}
