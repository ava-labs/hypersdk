// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"encoding/hex"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/cli"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/bls"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/crypto/secp256r1"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/auth"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	brpc "github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
	"github.com/ava-labs/hypersdk/spam"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/spf13/cobra"
)

func getFactory(priv *cli.PrivateKey) (chain.AuthFactory, error) {
	switch priv.Address[0] {
	case consts.ED25519ID:
		return auth.NewED25519Factory(ed25519.PrivateKey(priv.Bytes)), nil
	case consts.SECP256R1ID:
		return auth.NewSECP256R1Factory(secp256r1.PrivateKey(priv.Bytes)), nil
	case consts.BLSID:
		p, err := bls.PrivateKeyFromBytes(priv.Bytes)
		if err != nil {
			return nil, err
		}
		return auth.NewBLSFactory(p), nil
	default:
		return nil, ErrInvalidKeyType
	}
}

var spamCmd = &cobra.Command{
	Use: "spam",
	RunE: func(*cobra.Command, []string) error {
		return ErrMissingSubcommand
	},
}

var runSpamCmd = &cobra.Command{
	Use: "run [ed25519/secp256r1/bls]",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return ErrInvalidArgs
		}
		return checkKeyType(args[0])
	},
	RunE: func(_ *cobra.Command, args []string) error {
		var bclient *brpc.JSONRPCClient
		var pk *cli.PrivateKey
		if len(privateKey) > 0 {
			b, err := hex.DecodeString(privateKey)
			if err != nil {
				return err
			}
			pk = &cli.PrivateKey{
				Address: auth.NewED25519Address(ed25519.PrivateKey(b).PublicKey()),
				Bytes:   b,
			}
		}

		zipf := spam.NewZipfDistribution(sZipf, vZipf, plotZipf)
		keyEncodingType := args[0]
		spammer := spam.Spammer{
			H: handler.Root(),
			NumAccounts: numAccounts,
			TxsPerSecond: txsPerSecond,
			MinCapacity: minCapacity,
			StepSize: stepSize,

			NumClients: connsPerHost,
			ClusterInfo: clusterInfo,
			Hrp: consts.HRP,
			PrivateKey: pk,

			Zipf: zipf,

		}

		return spammer.Spam(
			func(uri string, networkID uint32, chainID ids.ID) error { // createClient
				bclient = brpc.NewJSONRPCClient(uri, networkID, chainID)
				return nil 
			},
			getFactory,
			func() (*cli.PrivateKey, error) { // createAccount
				return generatePrivateKey(keyEncodingType)
			},
			func(choice int, address string) (uint64, error) { // lookupBalance
				balance, err := bclient.Balance(context.TODO(), address)
				if err != nil {
					return 0, err
				}
				if choice != -1 {
					utils.Outf(
						"%d) {{cyan}}address:{{/}} %s {{cyan}}balance:{{/}} %s %s\n",
						choice,
						address,
						utils.FormatBalance(balance, consts.Decimals),
						consts.Symbol,
					)
				}
				return balance, err
			},
			func(ctx context.Context, chainID ids.ID) (chain.Parser, error) { // getParser
				return bclient.Parser(ctx)
			},
			func(addr codec.Address, create bool, amount uint64, memo []byte) chain.Action { // getTransfer
				return &actions.Transfer{
					To:     addr,
					Create: create,
					Value:  amount,
					Memo:   memo,
				}
			},
		)
	},
}
