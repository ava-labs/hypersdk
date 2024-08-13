package storage

import (
	"github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/database/corruptabledb"

	"github.com/ava-labs/hypersdk/pebble"
	"github.com/ava-labs/hypersdk/utils"
)

func New(cfg pebble.Config, chainDataDir string, namespace string, gatherer metrics.MultiGatherer) (database.Database, error) {
	path, err := utils.InitSubDirectory(chainDataDir, namespace)
	if err != nil {
		return nil, err
	}

	db, registry, err := pebble.New(path, cfg)
	if err != nil {
		return nil, err
	}

	if err := gatherer.Register(namespace, registry); err != nil {
		return nil, err
	}

	return corruptabledb.New(db), nil
}
