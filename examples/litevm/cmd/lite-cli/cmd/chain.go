// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

//nolint:lll
package cmd

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	runner "github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/math"
	hconsts "github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/window"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/ava-labs/hypersdk/examples/litevm/actions"
	"github.com/ava-labs/hypersdk/examples/litevm/auth"
	"github.com/ava-labs/hypersdk/examples/litevm/consts"
	trpc "github.com/ava-labs/hypersdk/examples/litevm/rpc"
	tutils "github.com/ava-labs/hypersdk/examples/litevm/utils"
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
		chainID, err := promptID("chainID")
		if err != nil {
			return err
		}
		uri, err := promptString("uri")
		if err != nil {
			return err
		}
		if err := StoreChain(chainID, uri); err != nil {
			return err
		}
		if err := StoreDefault(defaultChainKey, chainID[:]); err != nil {
			return err
		}
		return nil
	},
}

var importANRChainCmd = &cobra.Command{
	Use: "import-anr",
	RunE: func(_ *cobra.Command, args []string) error {
		ctx := context.Background()

		// Delete previous items
		oldChains, err := DeleteChains()
		if err != nil {
			return err
		}
		if len(oldChains) > 0 {
			utils.Outf("{{yellow}}deleted old chains:{{/}} %+v\n", oldChains)
		}

		// Load new items from ANR
		anrCli, err := runner.New(runner.Config{
			Endpoint:    "0.0.0.0:12352",
			DialTimeout: 10 * time.Second,
		}, logging.NoLog{})
		if err != nil {
			return err
		}
		status, err := anrCli.Status(ctx)
		if err != nil {
			return err
		}
		subnets := map[ids.ID][]ids.ID{}
		for chain, chainInfo := range status.ClusterInfo.CustomChains {
			chainID, err := ids.FromString(chain)
			if err != nil {
				return err
			}
			subnetID, err := ids.FromString(chainInfo.SubnetId)
			if err != nil {
				return err
			}
			chainIDs, ok := subnets[subnetID]
			if !ok {
				chainIDs = []ids.ID{}
			}
			chainIDs = append(chainIDs, chainID)
			subnets[subnetID] = chainIDs
		}
		var filledChainID ids.ID
		for _, nodeInfo := range status.ClusterInfo.NodeInfos {
			if len(nodeInfo.WhitelistedSubnets) == 0 {
				continue
			}
			trackedSubnets := strings.Split(nodeInfo.WhitelistedSubnets, ",")
			for _, subnet := range trackedSubnets {
				subnetID, err := ids.FromString(subnet)
				if err != nil {
					return err
				}
				for _, chainID := range subnets[subnetID] {
					uri := fmt.Sprintf("%s/ext/bc/%s", nodeInfo.Uri, chainID)
					if err := StoreChain(chainID, uri); err != nil {
						return err
					}
					utils.Outf(
						"{{yellow}}stored chainID:{{/}} %s {{yellow}}uri:{{/}} %s\n",
						chainID,
						uri,
					)
					filledChainID = chainID
				}
			}
		}
		return StoreDefault(defaultChainKey, filledChainID[:])
	},
}

type AvalancheOpsConfig struct {
	Resources struct {
		CreatedNodes []struct {
			HTTPEndpoint string `yaml:"httpEndpoint"`
		} `yaml:"created_nodes"`
	} `yaml:"resources"`
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
		// Delete previous items
		if deleteOtherChains {
			oldChains, err := DeleteChains()
			if err != nil {
				return err
			}
			if len(oldChains) > 0 {
				utils.Outf("{{yellow}}deleted old chains:{{/}} %+v\n", oldChains)
			}
		}

		// Load chainID
		chainID, err := ids.FromString(args[0])
		if err != nil {
			return err
		}

		// Load yaml file
		var opsConfig AvalancheOpsConfig
		yamlFile, err := os.ReadFile(args[1])
		if err != nil {
			return err
		}
		err = yaml.Unmarshal(yamlFile, &opsConfig)
		if err != nil {
			return err
		}

		// Add chains
		for _, node := range opsConfig.Resources.CreatedNodes {
			uri := fmt.Sprintf("%s/ext/bc/%s", node.HTTPEndpoint, chainID)
			if err := StoreChain(chainID, uri); err != nil {
				return err
			}
			utils.Outf(
				"{{yellow}}stored chainID:{{/}} %s {{yellow}}uri:{{/}} %s\n",
				chainID,
				uri,
			)
		}
		return StoreDefault(defaultChainKey, chainID[:])
	},
}

var setChainCmd = &cobra.Command{
	Use: "set",
	RunE: func(*cobra.Command, []string) error {
		chainID, _, err := promptChain("set default chain")
		if err != nil {
			return err
		}
		return StoreDefault(defaultChainKey, chainID[:])
	},
}

var chainInfoCmd = &cobra.Command{
	Use: "info",
	RunE: func(_ *cobra.Command, args []string) error {
		_, uris, err := promptChain("select chainID")
		if err != nil {
			return err
		}
		cli := rpc.NewJSONRPCClient(uris[0])
		networkID, subnetID, chainID, err := cli.Network(context.Background())
		if err != nil {
			return err
		}
		utils.Outf(
			"{{cyan}}networkID:{{/}} %d {{cyan}}subnetID:{{/}} %s {{cyan}}chainID:{{/}} %s",
			networkID,
			subnetID,
			chainID,
		)
		return nil
	},
}

var watchChainCmd = &cobra.Command{
	Use: "watch",
	RunE: func(_ *cobra.Command, args []string) error {
		ctx := context.Background()
		chainID, uris, err := promptChain("select chainID")
		if err != nil {
			return err
		}
		if err := CloseDatabase(); err != nil {
			return err
		}
		cli := trpc.NewJSONRPCClient(uris[0], chainID)
		utils.Outf("{{yellow}}uri:{{/}} %s\n", uris[0])
		scli, err := rpc.NewWebSocketClient(uris[0])
		if err != nil {
			return err
		}
		defer scli.Close()
		if err := scli.RegisterBlocks(); err != nil {
			return err
		}
		parser, err := cli.Parser(ctx)
		if err != nil {
			return err
		}
		utils.Outf("{{green}}watching for new blocks on %s 👀{{/}}\n", chainID)
		var (
			start             time.Time
			lastBlock         int64
			lastBlockDetailed time.Time
			tpsWindow         = window.Window{}
		)
		for ctx.Err() == nil {
			blk, results, err := scli.ListenBlock(ctx, parser)
			if err != nil {
				return err
			}
			now := time.Now()
			if start.IsZero() {
				start = now
			}
			if lastBlock != 0 {
				since := now.Unix() - lastBlock
				newWindow, err := window.Roll(tpsWindow, int(since))
				if err != nil {
					return err
				}
				tpsWindow = newWindow
				window.Update(&tpsWindow, window.WindowSliceSize-hconsts.Uint64Len, uint64(len(blk.Txs)))
				runningDuration := time.Since(start)
				tpsDivisor := math.Min(window.WindowSize, runningDuration.Seconds())
				utils.Outf(
					"{{green}}height:{{/}}%d {{green}}txs:{{/}}%d {{green}}units:{{/}}%d {{green}}root:{{/}}%s {{green}}TPS:{{/}}%.2f {{green}}split:{{/}}%dms\n", //nolint:lll
					blk.Hght,
					len(blk.Txs),
					blk.UnitsConsumed,
					blk.StateRoot,
					float64(window.Sum(tpsWindow))/tpsDivisor,
					time.Since(lastBlockDetailed).Milliseconds(),
				)
			} else {
				utils.Outf(
					"{{green}}height:{{/}}%d {{green}}txs:{{/}}%d {{green}}units:{{/}}%d {{green}}root:{{/}}%s\n", //nolint:lll
					blk.Hght,
					len(blk.Txs),
					blk.UnitsConsumed,
					blk.StateRoot,
				)
				window.Update(&tpsWindow, window.WindowSliceSize-hconsts.Uint64Len, uint64(len(blk.Txs)))
			}
			lastBlock = now.Unix()
			lastBlockDetailed = now
			if hideTxs {
				continue
			}
			for i, tx := range blk.Txs {
				result := results[i]
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
					"%s {{yellow}}%s{{/}} {{yellow}}root:{{/}} %s {{yellow}}actor:{{/}} %s {{yellow}}units:{{/}} %d {{yellow}}summary (%s):{{/}} [%s]\n",
					status,
					tx.ID(),
					tx.Proof.Root,
					tutils.Address(actor),
					result.Units,
					reflect.TypeOf(tx.Action),
					summaryStr,
				)
			}
		}
		return nil
	},
}
