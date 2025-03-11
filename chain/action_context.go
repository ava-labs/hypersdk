// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import "github.com/ava-labs/avalanchego/ids"

var _ ActionContext = (*ActionCtx)(nil)

type ActionCtx struct {
	height    uint64
	timestamp int64
	txID      ids.ID
}

func NewActionContext(height uint64, timestamp int64, txID ids.ID) *ActionCtx {
	return &ActionCtx{
		height:    height,
		timestamp: timestamp,
		txID:      txID,
	}
}

func (a *ActionCtx) GetHeight() uint64 {
	return a.height
}

func (a *ActionCtx) GetTimestamp() int64 {
	return a.timestamp
}

func (a *ActionCtx) GetTxID() ids.ID {
	return a.txID
}
