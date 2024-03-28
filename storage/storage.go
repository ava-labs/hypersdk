// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/database/corruptabledb"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/hypersdk/filedb"
	"github.com/ava-labs/hypersdk/pebble"
	"github.com/ava-labs/hypersdk/utils"
)

// New creates a VM database, a Blob database, and a State database. The VM and Blob databases fsync all writes to
// disk whereas the State database does not. If there is an unclean shutdown, we can recover the State database
// by reprocessing data from the Blob database. The VM database stores which blocks we have already accepted
// and consensus integrity depends on that being correct (can't be recovered using other mechanisms).
func New(chainDataDir string, gatherer metrics.MultiGatherer) (database.Database, *filedb.FileDB, database.Database, error) {
	vmCfg := pebble.NewDefaultConfig()
	vmCfg.Sync = true
	vmPath, err := utils.InitSubDirectory(chainDataDir, vm)
	if err != nil {
		return nil, nil, nil, err
	}
	vmDB, vmDBRegistry, err := pebble.New(vmPath, vmCfg)
	if err != nil {
		return nil, nil, nil, err
	}
	if gatherer != nil {
		if err := gatherer.Register(vm, vmDBRegistry); err != nil {
			return nil, nil, nil, err
		}
	}
	blobPath, err := utils.InitSubDirectory(chainDataDir, blob)
	blobDB := filedb.New(blobPath, true, 1024, 512*units.MiB) // TODO: make configurable
	statePath, err := utils.InitSubDirectory(chainDataDir, state)
	if err != nil {
		return nil, nil, nil, err
	}
	stateCfg := pebble.NewDefaultConfig()
	stateCfg.Sync = false // we can recover invalid state from persisted blocks/chunks
	stateDB, stateDBRegistry, err := pebble.New(statePath, stateCfg)
	if err != nil {
		return nil, nil, nil, err
	}
	if gatherer != nil {
		if err := gatherer.Register(state, stateDBRegistry); err != nil {
			return nil, nil, nil, err
		}
	}
	return corruptabledb.New(vmDB), blobDB, corruptabledb.New(stateDB), nil
}
