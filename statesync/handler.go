// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package statesync

import (
	"errors"

	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/ava-labs/avalanchego/x/sync"
)

func RegisterHandlers(
	log logging.Logger,
	network *p2p.Network,
	rangeProofHandlerID uint64,
	changeProofHandlerID uint64,
	db merkledb.MerkleDB,
) error {
	return errors.Join(
		network.AddHandler(
			rangeProofHandlerID,
			sync.NewGetRangeProofHandler(log, db),
		),
		network.AddHandler(
			changeProofHandlerID,
			sync.NewGetChangeProofHandler(log, db),
		),
	)
}
