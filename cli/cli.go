// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cli

import (
	"github.com/ava-labs/avalanchego/database"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/internal/pebble"
)

type Controller interface {
	DatabasePath() string
	Symbol() string
	GetParser() chain.Parser
	HandleTx(*chain.Transaction, *chain.Result)
	LookupBalance(address codec.Address, uri string) (uint64, error)
}

type Handler struct {
	c Controller

	db database.Database
}

func New(c Controller) (*Handler, error) {
	db, err := pebble.New(c.DatabasePath(), pebble.NewDefaultConfig(), prometheus.NewRegistry())
	if err != nil {
		return nil, err
	}
	return &Handler{c, db}, nil
}
