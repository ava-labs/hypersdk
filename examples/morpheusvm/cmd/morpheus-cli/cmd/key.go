// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/crypto/secp256r1"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/auth"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	brpc "github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/spf13/cobra"
)

const (
	ed25519Key   = "ed25519"
	secp256r1Key = "secp256r1"
)

var keyCmd = &cobra.Command{
	Use: "key",
	RunE: func(*cobra.Command, []string) error {
		return ErrMissingSubcommand
	},
}

var genKeyCmd = &cobra.Command{
	Use: "generate [ed25519/secp256r1]",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return ErrInvalidArgs
		}
		if args[0] != ed25519Key && args[0] != secp256r1Key {
			return fmt.Errorf("%w: %s", ErrInvalidKeyType, args[0])
		}
		return nil
	},
	RunE: func(_ *cobra.Command, args []string) error {
		var (
			addr codec.AddressBytes
			priv []byte
		)
		switch args[0] {
		case ed25519Key:
			p, err := ed25519.GeneratePrivateKey()
			if err != nil {
				return err
			}
			priv = p[:]
			addr = auth.NewED25519Address(p.PublicKey())
		case secp256r1Key:
			p, err := secp256r1.GeneratePrivateKey()
			if err != nil {
				return err
			}
			priv = p[:]
			addr = auth.NewSECP256R1Address(p.PublicKey())
		}
		if err := handler.h.StoreKey(addr, priv); err != nil {
			return err
		}
		if err := handler.h.StoreDefaultKey(addr); err != nil {
			return err
		}
		utils.Outf(
			"{{green}}created address:{{/}} %s",
			codec.MustAddress(consts.HRP, addr),
		)
		return nil
	},
}

var importKeyCmd = &cobra.Command{
	Use: "import [type] [path]",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 2 {
			return ErrInvalidArgs
		}
		if args[0] != ed25519Key && args[0] != secp256r1Key {
			return fmt.Errorf("%w: %s", ErrInvalidKeyType, args[0])
		}
		return nil
	},
	RunE: func(_ *cobra.Command, args []string) error {
		var (
			addr codec.AddressBytes
			priv []byte
		)
		switch args[0] {
		case ed25519Key:
			p, err := utils.LoadBytes(args[1], ed25519.PrivateKeyLen)
			if err != nil {
				return err
			}
			pk := ed25519.PrivateKey(p)
			priv = p[:]
			addr = auth.NewED25519Address(pk.PublicKey())
		case secp256r1Key:
			p, err := utils.LoadBytes(args[1], secp256r1.PrivateKeyLen)
			if err != nil {
				return err
			}
			pk := secp256r1.PrivateKey(p)
			priv = p[:]
			addr = auth.NewSECP256R1Address(pk.PublicKey())
		}
		if err := handler.h.StoreKey(addr, priv); err != nil {
			return err
		}
		if err := handler.h.StoreDefaultKey(addr); err != nil {
			return err
		}
		utils.Outf(
			"{{green}}imported address:{{/}} %s",
			codec.MustAddress(consts.HRP, addr),
		)
		return nil
	},
}

func lookupSetKeyBalance(choice int, address string, uri string, networkID uint32, chainID ids.ID) error {
	// TODO: just load once
	cli := brpc.NewJSONRPCClient(uri, networkID, chainID)
	balance, err := cli.Balance(context.TODO(), address)
	if err != nil {
		return err
	}
	utils.Outf(
		"%d) {{cyan}}address:{{/}} %s {{cyan}}balance:{{/}} %s %s\n",
		choice,
		address,
		utils.FormatBalance(balance, consts.Decimals),
		consts.Symbol,
	)
	return nil
}

var setKeyCmd = &cobra.Command{
	Use: "set",
	RunE: func(*cobra.Command, []string) error {
		return handler.Root().SetKey(lookupSetKeyBalance)
	},
}

func lookupKeyBalance(addr codec.AddressBytes, uri string, networkID uint32, chainID ids.ID, _ ids.ID) error {
	_, err := handler.GetBalance(context.TODO(), brpc.NewJSONRPCClient(uri, networkID, chainID), addr)
	return err
}

var balanceKeyCmd = &cobra.Command{
	Use: "balance",
	RunE: func(*cobra.Command, []string) error {
		return handler.Root().Balance(checkAllChains, false, lookupKeyBalance)
	},
}
