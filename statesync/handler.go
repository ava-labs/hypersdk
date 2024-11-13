// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package statesync

import (
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/ava-labs/avalanchego/x/sync"
)

func RegisterHandlers(log logging.Logger, network *p2p.Network, db merkledb.MerkleDB) error {
	errs := wrappers.Errs{}
	errs.Add(
		network.AddHandler(
			RangeProofHandlerID,
			sync.NewGetRangeProofHandler(log, db),
		),
		network.AddHandler(
			ChangeProofHandlerID,
			sync.NewGetChangeProofHandler(log, db),
		),
	)
	return errs.Err
}
