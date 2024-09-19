// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cli

import (
	"github.com/ava-labs/avalanchego/database"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/internal/pebble"
)

type Controller[T chain.RuntimeInterface] interface {
	DatabasePath() string
	Symbol() string
	Decimals() uint8
	GetParser(string) (chain.Parser[T], error)
	HandleTx(*chain.Transaction[T], *chain.Result)
	LookupBalance(address codec.Address, uri string) (uint64, error)
}

type Handler[T chain.RuntimeInterface] struct {
	c Controller[T]

	db database.Database
}

func New[T chain.RuntimeInterface](c Controller[T]) (*Handler[T], error) {
	db, _, err := pebble.New(c.DatabasePath(), pebble.NewDefaultConfig())
	if err != nil {
		return nil, err
	}
	return &Handler[T]{c, db}, nil
}
