package merkle

import (
	"context"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/ava-labs/hypersdk/utils"
)

// Generate merkle root for a set of items
func GenerateMerkleRoot(ctx context.Context, tracer trace.Tracer, merkleItems [][]byte, consumeBytes bool) (ids.ID, merkledb.MerkleDB, error) {
	batchOps := make([]database.BatchOp, 0, len(merkleItems))

	for _, item := range merkleItems {
		key := utils.ToID(item)
		batchOps = append(batchOps, database.BatchOp{
			Key:   key[:],
			Value: item,
		})
	}

	db, err := merkledb.New(ctx, memdb.New(), merkledb.Config{
		BranchFactor:              merkledb.BranchFactor16,
		HistoryLength:             100,
		IntermediateNodeCacheSize: units.MiB,
		ValueNodeCacheSize:        units.MiB,
		Tracer:                    tracer,
	})
	if err != nil {
		return ids.Empty, nil, err
	}

	view, err := db.NewView(ctx, merkledb.ViewChanges{BatchOps: batchOps, ConsumeBytes: consumeBytes})
	if err != nil {
		return ids.Empty, nil, err
	}
	if err := view.CommitToDB(ctx); err != nil {
		return ids.Empty, nil, err
	}

	root, err := db.GetMerkleRoot(ctx)
	if err != nil {
		return ids.Empty, nil, err
	}

	return root, db, nil
}
