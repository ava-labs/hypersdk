// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package merkle

import (
	"context"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/database/memdb"
<<<<<<< HEAD
	"github.com/ava-labs/avalanchego/ids"
=======
	"github.com/ava-labs/avalanchego/trace"
>>>>>>> f2bb2f97 (Revert "rebase & blk marshal/unmarshal & merkleroot to ids.ID")
	"github.com/ava-labs/avalanchego/x/merkledb"

	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/avalanchego/ids"
)

// Generate merkle root for a set of items
func GenerateMerkleRoot(ctx context.Context, config merkledb.Config, merkleItems [][]byte, consumeBytes bool) (ids.ID, merkledb.MerkleDB, error) {
	batchOps := make([]database.BatchOp, 0, len(merkleItems))

	for _, item := range merkleItems {
		key := utils.ToID(item)
		batchOps = append(batchOps, database.BatchOp{
			Key:   key[:],
			Value: item,
		})
	}

	db, err := merkledb.New(ctx, memdb.New(), config)
	if err != nil {
		return ids.Empty, nil, err
	}

	view, err := db.NewView(ctx, merkledb.ViewChanges{BatchOps: batchOps, ConsumeBytes: consumeBytes})
	if err != nil {
		return ids.Empty, nil, err
	}

	root, err := view.GetMerkleRoot(ctx)
	if err != nil {
		return ids.Empty, nil, err
	}

	return root, db, nil
}
