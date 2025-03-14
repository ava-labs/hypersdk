// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package ws

import (
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
)

//go:generate go run github.com/StephenButtolph/canoto/canoto $GOFILE

const (
	BlockMode byte = 0
	TxMode    byte = 1
)

type txMessage struct {
	TxID        ids.ID `canoto:"fixed bytes,1"`
	ResultBytes []byte `canoto:"bytes,2"`

	canotoData canotoData_txMessage
}

// packTxMessage packs a txID and result. A nil result indicates the tx was
// marked as expired.
// Expiry is the only failure condition that triggers a notification sent to
// the client.
func packTxMessage(txID ids.ID, result *chain.Result) []byte {
	var resultBytes []byte
	if result != nil {
		resultBytes = result.Marshal()
	}
	txMessage := txMessage{
		TxID:        txID,
		ResultBytes: resultBytes,
	}
	txMessageBytes := txMessage.MarshalCanoto()
	return txMessageBytes
}

// unpackTxMessage unpacks a txID and result. A nil result indicates the
// tx was marked as expired (only failure condition that triggers a notification).
func unpackTxMessage(txMsgBytes []byte) (ids.ID, *chain.Result, error) {
	txMessage := txMessage{}
	if err := txMessage.UnmarshalCanoto(txMsgBytes); err != nil {
		return ids.Empty, nil, err
	}
	var (
		result *chain.Result
		err    error
	)
	if len(txMessage.ResultBytes) > 0 {
		result, err = chain.UnmarshalResult(txMessage.ResultBytes)
		if err != nil {
			return ids.Empty, nil, err
		}
	}
	return txMessage.TxID, result, nil
}
