// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/vm"
	"github.com/ava-labs/hypersdk/examples/tokenvm/client"
	"github.com/spf13/cobra"
)

var watchCmd = &cobra.Command{
	Use:   "watch [options]",
	Short: "Watch monitors network activity",
	RunE:  watchFunc,
}

func watchFunc(_ *cobra.Command, _ []string) error {
	ctx := context.Background()
	cli := client.New(uri)
	port, err := cli.BlocksPort(ctx)
	if err != nil {
		return err
	}
	host, err := utils.GetHost(uri)
	if err != nil {
		return err
	}
	scli, err := vm.NewBlockRPCClient(fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return err
	}
	defer scli.Close()
	parser, err := cli.Parser(ctx)
	if err != nil {
		return err
	}
	totalTxs := float64(0)
	start := time.Now()
	utils.Outf("{{green}}watching for new blocks 👀{{/}}\n")
	for ctx.Err() == nil {
		blk, _, err := scli.Listen(parser)
		if err != nil {
			return err
		}
		totalTxs += float64(len(blk.Txs))
		utils.Outf(
			"{{yellow}}height:{{/}}%d {{yellow}}txs:{{/}}%d {{yellow}}units:{{/}}%d {{yellow}}root:{{/}}%s {{yellow}}avg TPS:{{/}}%f\n", //nolint:lll
			blk.Hght,
			len(blk.Txs),
			blk.UnitsConsumed,
			blk.StateRoot,
			totalTxs/time.Since(start).Seconds(),
		)
	}
	return nil
}
