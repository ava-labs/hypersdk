// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/spf13/cobra"

	brpc "github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
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

var importAvalancheCliChainCmd = &cobra.Command{
	Use: "import-cli [path]",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return ErrInvalidArgs
		}
		return nil
	},
	RunE: func(_ *cobra.Command, args []string) error {
		return handler.Root().ImportCLI(args[0])
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

var watchChainCmd = &cobra.Command{
	Use: "watch",
	RunE: func(_ *cobra.Command, args []string) error {
		return handler.Root().WatchChain(hideTxs, func(uri string, networkID uint32, chainID ids.ID) (chain.Parser, error) {
			cli := brpc.NewJSONRPCClient(uri, networkID, chainID)
			return cli.Parser(context.TODO())
		}, handleTx)
	},
}

var watchPreConfsCmd = &cobra.Command{
	Use: "watch-preconfs",
	RunE: func(_ *cobra.Command, args []string) error {
		ctx := context.Background()
		_, _, _, cli, _, ws, err := handler.DefaultActor()
		if err != nil {
			return err
		}
		if err := handler.h.CloseDatabase(); err != nil {
			return err
		}
		if err := ws.RegisterPreConf(); err != nil {
			return err
		}
		_, chainID, _, err := cli.Network(ctx)
		if err != nil {
			return err
		}
		utils.Outf("{{green}}watching for preconf on %s 👀{{/}}\n", chainID)
		for ctx.Err() == nil {
			chunkID, err := ws.ListenPreConf(ctx)
			if err != nil {
				return err
			}
			// _, h, _, _ := cli.Accepted(ctx)
			utils.Outf("new preconf issued: %s at current block height, time: %d\n", chunkID, time.Now().UnixMilli())
		}
		return nil
	},
}

var watchPreConfsStandCmd = &cobra.Command{
	Use: "watch-preconfs-stand",
	RunE: func(_ *cobra.Command, args []string) error {
		ctx := context.Background()
		_, _, _, cli, _, ws, err := handler.DefaultActor()
		if err != nil {
			return err
		}
		if err := handler.h.CloseDatabase(); err != nil {
			return err
		}
		if err := ws.RegisterBlocks(); err != nil {
			return err
		}
		_, chainID, _, err := cli.Network(ctx)
		if err != nil {
			return err
		}
		utils.Outf("{{green}}watching for preconf on %s 👀{{/}}\n", chainID)
		for ctx.Err() == nil {
			blk, chunkIDs, err := ws.ListenBlock(ctx)
			if err != nil {
				return err
			}
			utils.Outf("received new block: %d at time %d\n", blk.Height, time.Now().UnixMilli())
			for _, chunkID := range chunkIDs {
				utils.Outf("preconf honoured: %s at block height: %d\n", chunkID, blk.Height)
			}
		}
		return nil
	},
}
