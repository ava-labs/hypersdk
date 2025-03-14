// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pubsub

//go:generate go run github.com/StephenButtolph/canoto/canoto $GOFILE

type BatchMessage struct {
	Messages [][]byte `canoto:"repeated bytes,1"`

	canotoData canotoData_BatchMessage
}

func CreateBatchMessage(msgs [][]byte) []byte {
	batchMessage := BatchMessage{Messages: msgs}
	return batchMessage.MarshalCanoto()
}

func ParseBatchMessage(msg []byte) ([][]byte, error) {
	batchMessage := BatchMessage{}
	if err := batchMessage.UnmarshalCanoto(msg); err != nil {
		return nil, err
	}
	return batchMessage.Messages, nil
}
