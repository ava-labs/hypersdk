// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"fmt"

	"github.com/akamensky/argparse"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/storage"
)

var _ Cmd = (*keyCreateCmd)(nil)

type keyCreateCmd struct {
	cmd *argparse.Command

	log  logging.Logger
	db   **state.SimpleMutable
	name *string
}

func (c *keyCreateCmd) New(parser *argparse.Parser, db **state.SimpleMutable) Cmd {
	cmd := &keyCreateCmd{}
	cmd.db = db
	cmd.cmd = parser.NewCommand("key-create", "Creates a new named private key and stores it in the database")
	cmd.name = cmd.cmd.String("", "name", &argparse.Options{Required: true})
	c = cmd

	return cmd
}

func (c *keyCreateCmd) Run(ctx context.Context, log logging.Logger, args []string) error {
	_, err := keyCreateFunc(ctx, *c.db, *c.name)
	if err != nil {
		return err
	}
	return nil
}

func (c *keyCreateCmd) Happened() bool {
	return c.cmd.Happened()
}

func keyCreateFunc(ctx context.Context, db *state.SimpleMutable, name string) (ed25519.PublicKey, error) {
	priv, err := ed25519.GeneratePrivateKey()
	if err != nil {
		return ed25519.EmptyPublicKey, err
	}
	ok, err := hasKey(ctx, db, name)
	if ok {
		return ed25519.EmptyPublicKey, fmt.Errorf("%w: %s", ErrDuplicateKeyName, name)
	}
	if err != nil {
		return ed25519.EmptyPublicKey, err
	}
	err = storage.SetKey(ctx, db, priv, name)
	if err != nil {
		return ed25519.EmptyPublicKey, err
	}

	err = db.Commit(ctx)
	if err != nil {
		return ed25519.EmptyPublicKey, err
	}

	return priv.PublicKey(), nil
}

func hasKey(ctx context.Context, db state.Immutable, name string) (bool, error) {
	_, ok, err := storage.GetPublicKey(ctx, db, name)
	return ok, err
}
