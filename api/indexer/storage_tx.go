// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package indexer

//go:generate go run github.com/StephenButtolph/canoto/canoto $GOFILE

type storageTx struct {
	Timestamp int64    `canoto:"int,1"`
	Success   bool     `canoto:"bool,2"`
	Units     []byte   `canoto:"bytes,3"`
	Fee       uint64   `canoto:"int,4"`
	Outputs   [][]byte `canoto:"repeated bytes,5"`
	Error     string   `canoto:"string,6"`

	canotoData canotoData_storageTx
}
