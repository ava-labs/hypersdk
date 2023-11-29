// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package program

type Instance interface {
	GetFunc(name string) (*Func, error)
	GetExport(name string) (*Export, error)
	Memory() (*Memory, error)
}
