// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/state"

	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/auth"
	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/storage"
)

func newKeyCmd(log logging.Logger, db *state.SimpleMutable) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "key",
		Short: "Manage private keys",
	}
	cmd.AddCommand(
		newKeyCreateCmd(log, db),
	)
	return cmd
}

type keyCreateCmd struct {
	log  logging.Logger
	db   *state.SimpleMutable
	name string
}

func newKeyCreateCmd(log logging.Logger, db *state.SimpleMutable) *cobra.Command {
	c := &keyCreateCmd{
		log: log,
		db:  db,
	}

	return &cobra.Command{
		Use:   "create [name]",
		Short: "Creates a new named private key and stores it in the database",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := c.Init(args)
			if err != nil {
				return err
			}
			err = c.Verify()
			if err != nil {
				return err
			}
			return c.Run(cmd.Context())
		},
	}
}

func (c *keyCreateCmd) Init(args []string) error {
	c.name = args[0]
	return nil
}

func (c *keyCreateCmd) Verify() error {
	return nil
}

func (c *keyCreateCmd) Run(ctx context.Context) error {
	_, err := authCreateFunc(ctx, c.db, c.name)
	if err != nil {
		return err
	}
	return nil
}

func authCreateFunc(ctx context.Context, db *state.SimpleMutable, name string) (codec.Address, error) {
	priv, err := ed25519.GeneratePrivateKey()
	if err != nil {
		return codec.EmptyAddress, err
	}
	ok, err := hasAddress(ctx, db, name)
	if ok {
		return codec.EmptyAddress, fmt.Errorf("%w: %s", ErrDuplicateKeyName, name)
	}
	if err != nil {
		return codec.EmptyAddress, err
	}
	err = storage.SetKey(ctx, db, priv, name)
	if err != nil {
		return codec.EmptyAddress, err
	}

	err = db.Commit(ctx)
	if err != nil {
		return codec.EmptyAddress, err
	}

	return auth.NewED25519Address(priv.PublicKey()), nil
}

func hasAddress(ctx context.Context, db state.Immutable, name string) (bool, error) {
	_, ok, err := storage.GetAddress(ctx, db, name)
	return ok, err
}
