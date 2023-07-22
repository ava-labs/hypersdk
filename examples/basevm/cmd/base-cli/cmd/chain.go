// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

//nolint:lll
package cmd

import (
	"context"
	"fmt"
	"reflect"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/examples/basevm/actions"
	"github.com/ava-labs/hypersdk/examples/basevm/auth"
	"github.com/ava-labs/hypersdk/examples/basevm/consts"
	brpc "github.com/ava-labs/hypersdk/examples/basevm/rpc"
	tutils "github.com/ava-labs/hypersdk/examples/basevm/utils"
)

var chainCmd = &cobra.Command{
	Use: "chain",
	RunE: func(*cobra.Command, []string) error {
		return ErrMissingSubcommand
	},
}

var importChainCmd = &cobra.Command{
	Use: "import",
	RunE: func(_ *cobra.Command, args []string) error {
		return handler.Root().ImportChain()
	},
}

var importANRChainCmd = &cobra.Command{
	Use: "import-anr",
	RunE: func(_ *cobra.Command, args []string) error {
		return handler.Root().ImportANR()
	},
}

var importAvalancheOpsChainCmd = &cobra.Command{
	Use: "import-ops [chainID] [path]",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 2 {
			return ErrInvalidArgs
		}
		_, err := ids.FromString(args[0])
		return err
	},
	RunE: func(_ *cobra.Command, args []string) error {
		return handler.Root().ImportOps(args[0], args[1])
	},
}

var setChainCmd = &cobra.Command{
	Use: "set",
	RunE: func(*cobra.Command, []string) error {
		return handler.Root().SetDefaultChain()
	},
}

var chainInfoCmd = &cobra.Command{
	Use: "info",
	RunE: func(_ *cobra.Command, args []string) error {
		return handler.Root().PrintChainInfo()
	},
}

func handleTx(tx *chain.Transaction, result *chain.Result) {
	summaryStr := string(result.Output)
	actor := auth.GetActor(tx.Auth)
	status := "⚠️"
	if result.Success {
		status = "✅"
		switch action := tx.Action.(type) {
		case *actions.Transfer:
			summaryStr = fmt.Sprintf("%s %s -> %s", utils.FormatBalance(action.Value), consts.Symbol, tutils.Address(action.To))
		}
	}
	utils.Outf(
		"%s {{yellow}}%s{{/}} {{yellow}}actor:{{/}} %s {{yellow}}units:{{/}} %d {{yellow}}summary (%s):{{/}} [%s]\n",
		status,
		tx.ID(),
		tutils.Address(actor),
		result.Units,
		reflect.TypeOf(tx.Action),
		summaryStr,
	)
}

var watchChainCmd = &cobra.Command{
	Use: "watch",
	RunE: func(_ *cobra.Command, args []string) error {
		return handler.Root().WatchChain(hideTxs, func(uri string, networkID uint32, chainID ids.ID) (chain.Parser, error) {
			cli := brpc.NewJSONRPCClient(uri, networkID, chainID)
			return cli.Parser(context.TODO())
		}, handleTx)
	},
}
