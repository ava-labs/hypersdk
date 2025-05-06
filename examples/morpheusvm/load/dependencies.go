// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package load

import "github.com/ava-labs/avalanchego/ids"

// Tracker keeps track of the status of transactions.
type Tracker interface {
	// Issue records a transaction that was submitted, but whose final status is
	// not yet known.
	Issue(ids.ID)
}
