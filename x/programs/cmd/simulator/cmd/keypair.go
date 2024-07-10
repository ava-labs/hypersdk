// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/akamensky/argparse"
	"github.com/ava-labs/avalanchego/utils/logging"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/state"
)

var _ Cmd = (*keypairCreateCmd)(nil)

type keypairCreateCmd struct {
	cmd *argparse.Command

	log  logging.Logger
	seed *string
}

func (c *keypairCreateCmd) New(parser *argparse.Parser) {
	c.cmd = parser.NewCommand("keypair-create", "Creates a new keypair")
	c.seed = c.cmd.String("", "seed", &argparse.Options{Required: false})
}

func (c *keypairCreateCmd) Run(_ context.Context, log logging.Logger, _ *state.SimpleMutable, _ []string) (*Response, error) {
	resp := newResponse(0)
	resp.setTimestamp(time.Now().Unix())
	c.log = log
	pkey, err := keypairCreate(*c.seed)
	if err != nil {
		return resp, err
	}

	c.log.Debug("keypair create successful", zap.String("public key", fmt.Sprintf("%x", pkey)))

	return resp, nil
}

func (c *keypairCreateCmd) Happened() bool {
	return c.cmd.Happened()
}

func keypairCreate(seed string) (ed25519.PublicKey, error) {
	var priv ed25519.PrivateKey

	if len(seed) != 0 {
		hexString := strings.TrimPrefix(seed, "0x")
		data, err := hex.DecodeString(hexString)
		if err != nil {
			return ed25519.EmptyPublicKey, err
		}
		priv, err = ed25519.GeneratePrivateKeyFromSeed(data)
		if err != nil {
			return ed25519.EmptyPublicKey, err
		}
	} else {
		var err error
		priv, err = ed25519.GeneratePrivateKey()
		if err != nil {
			return ed25519.EmptyPublicKey, err
		}
	}

	return priv.PublicKey(), nil
}
