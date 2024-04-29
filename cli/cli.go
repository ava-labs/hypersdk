// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cli

import (
	"github.com/ava-labs/avalanchego/database"

	"github.com/ava-labs/hypersdk/pebble"
)

type Handler struct {
	c Controller

	db database.Database
}

func New(c Controller) (*Handler, error) {
	db, _, err := pebble.New(c.DatabasePath(), pebble.NewDefaultConfig())
	if err != nil {
		return nil, err
	}
	return &Handler{c, db}, nil
}
