// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package v2

type Key []byte
type AccountID string

type AccountConfig struct {
}

type Account struct {
	AccountID AccountID
	balance   uint64
	programID ProgramID
}
