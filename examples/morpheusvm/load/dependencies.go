// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package load

import "github.com/ava-labs/avalanchego/ids"

// Tracker keeps track of the status of transactions.
// This must be thread-safe, so it can be called in parallel by the issuer(s) or orchestrator.
type Tracker interface {
	// Issue records a transaction that was submitted, but whose final status is
	// not yet known.
	Issue(ids.ID)
	// ObserveConfirmed records a transaction that was confirmed.
	ObserveConfirmed(ids.ID)
	// ObserveFailed records a transaction that failed (e.g. expired)
	ObserveFailed(ids.ID)

	// GetObservedIssued returns the number of transactions that the tracker has
	// confirmed were issued.
	GetObservedIssued() uint64
	// GetObservedConfirmed returns the number of transactions that the tracker has
	// confirmed were accepted.
	GetObservedConfirmed() uint64
	// GetObservedFailed returns the number of transactions that the tracker has
	// confirmed failed.
	GetObservedFailed() uint64
}
