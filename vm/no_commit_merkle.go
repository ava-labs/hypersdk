// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"

	"github.com/ava-labs/avalanchego/x/merkledb"
)

// MerkleDB implements CommitToDB as a no-op, but it does not expose it as part of the exported interface,
// so we provide it here.
// TODO: add this directly to the MerkleDB interface.
type MerkleDBWithNoopCommit struct {
	merkledb.MerkleDB
}

func (MerkleDBWithNoopCommit) CommitToDB(context.Context) error { return nil }
