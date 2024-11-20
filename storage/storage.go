// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/database/corruptabledb"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ava-labs/hypersdk/internal/pebble"
	"github.com/ava-labs/hypersdk/utils"
)

func New(cfg pebble.Config, chainDataDir string, namespace string, registerer prometheus.Registerer) (database.Database, error) {
	path, err := utils.InitSubDirectory(chainDataDir, namespace)
	if err != nil {
		return nil, err
	}

	db, err := pebble.New(path, cfg, registerer)
	if err != nil {
		return nil, err
	}

	return corruptabledb.New(db), nil
}
