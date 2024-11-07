// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/crypto/bls"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/crypto/secp256r1"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/vm"
	hthroughput "github.com/ava-labs/hypersdk/throughput"
)

var spamCmd = &cobra.Command{
	Use: "spam",
	RunE: func(*cobra.Command, []string) error {
		return ErrMissingSubcommand
	},
}

var runSpamCmd = &cobra.Command{
	Use: "run [ed25519/secp256r1/bls]",
	PreRunE: func(_ *cobra.Command, args []string) error {
		if len(args) != 1 {
			return ErrInvalidArgs
		}
		return vm.AuthProvider.CheckType(args[0])
	},
	RunE: func(_ *cobra.Command, args []string) error {
		ctx := context.Background()
		privateKey := "323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7"
		key, err := generatePrivateKey(privateKey)
		if err != nil {
			return err
		}
		chains, err := handler.Root().GetChains()
		if err != nil {
			return err
		}

		keys := make([]ids.ID, 0, len(chains))
		for chainID := range chains {
			keys = append(keys, chainID)
		}
		chainIndex := 0
		chainID := keys[chainIndex]
		uris := chains[chainID]
	

		sc := hthroughput.NewDefaultConfig(uris, key)
		spammer, err := hthroughput.NewSpammer(sc, sh)
		if err != nil {
			return err
		}

	
		return spammer.Spam(ctx, sh, false, h.c.Symbol())
	},
}

func generatePrivateKey(k string) (*cli.PrivateKey, error) {
	switch k {
	case ed25519Key:
		p, err := ed25519.GeneratePrivateKey()
		if err != nil {
			return nil, err
		}
		return &cli.PrivateKey{
			Address: auth.NewED25519Address(p.PublicKey()),
			Bytes:   p[:],
		}, nil
	case secp256r1Key:
		p, err := secp256r1.GeneratePrivateKey()
		if err != nil {
			return nil, err
		}
		return &cli.PrivateKey{
			Address: auth.NewSECP256R1Address(p.PublicKey()),
			Bytes:   p[:],
		}, nil
	case blsKey:
		p, err := bls.GeneratePrivateKey()
		if err != nil {
			return nil, err
		}
		return &cli.PrivateKey{
			Address: auth.NewBLSAddress(bls.PublicFromPrivateKey(p)),
			Bytes:   bls.PrivateKeyToBytes(p),
		}, nil
	default:
		return nil, ErrInvalidKeyType
	}
}
