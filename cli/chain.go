// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/units"
	"gopkg.in/yaml.v2"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/window"
)

func (h *Handler) ImportChain() error {
	chainID, err := h.PromptID("chainID")
	if err != nil {
		return err
	}
	uri, err := h.PromptString("uri", 0, consts.MaxInt)
	if err != nil {
		return err
	}
	if err := h.StoreChain(chainID, uri); err != nil {
		return err
	}
	if err := h.StoreDefaultChain(chainID); err != nil {
		return err
	}
	return nil
}

func (h *Handler) ImportANR() error {
	ctx := context.Background()

	// Delete previous items
	oldChains, err := h.DeleteChains()
	if err != nil {
		return err
	}
	if len(oldChains) > 0 {
		utils.Outf("{{yellow}}deleted old chains:{{/}} %+v\n", oldChains)
	}

	// Load new items from ANR
	anrCli, err := client.New(client.Config{
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
				if err := h.StoreChain(chainID, uri); err != nil {
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
	return h.StoreDefaultChain(filledChainID)
}

type AvalancheOpsConfig struct {
	Resources struct {
		CreatedNodes []struct {
			HTTPEndpoint string `yaml:"httpEndpoint"`
		} `yaml:"created_nodes"`
	} `yaml:"resource"`
	VMInstall struct {
		ChainID string `yaml:"chain_id"`
	} `yaml:"vm_install"`
}

func (h *Handler) ImportOps(opsPath string) error {
	oldChains, err := h.DeleteChains()
	if err != nil {
		return err
	}
	if len(oldChains) > 0 {
		utils.Outf("{{yellow}}deleted old chains:{{/}} %+v\n", oldChains)
	}

	// Load yaml file
	var opsConfig AvalancheOpsConfig
	yamlFile, err := os.ReadFile(opsPath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(yamlFile, &opsConfig)
	if err != nil {
		return err
	}

	// Load chainID
	chainID, err := ids.FromString(opsConfig.VMInstall.ChainID)
	if err != nil {
		return err
	}

	// Add chains
	for _, node := range opsConfig.Resources.CreatedNodes {
		uri := fmt.Sprintf("%s/ext/bc/%s", node.HTTPEndpoint, chainID)
		if err := h.StoreChain(chainID, uri); err != nil {
			return err
		}
		utils.Outf(
			"{{yellow}}stored chainID:{{/}} %s {{yellow}}uri:{{/}} %s\n",
			chainID,
			uri,
		)
	}
	return h.StoreDefaultChain(chainID)
}

func (h *Handler) SetDefaultChain() error {
	chainID, _, err := h.PromptChain("set default chain", nil)
	if err != nil {
		return err
	}
	return h.StoreDefaultChain(chainID)
}

func (h *Handler) PrintChainInfo() error {
	_, uris, err := h.PromptChain("select chainID", nil)
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
}

func (h *Handler) WatchChain(hideTxs bool, getParser func(string, uint32, ids.ID) (chain.Parser, error), handleTx func(*chain.Transaction, *chain.Result)) error {
	ctx := context.Background()
	chainID, uris, err := h.PromptChain("select chainID", nil)
	if err != nil {
		return err
	}
	if err := h.CloseDatabase(); err != nil {
		return err
	}
	utils.Outf("{{yellow}}uri:{{/}} %s\n", uris[0])
	rcli := rpc.NewJSONRPCClient(uris[0])
	networkID, _, _, err := rcli.Network(context.TODO())
	if err != nil {
		return err
	}
	parser, err := getParser(uris[0], networkID, chainID)
	if err != nil {
		return err
	}
	scli, err := rpc.NewWebSocketClient(uris[0], rpc.DefaultHandshakeTimeout, pubsub.MaxPendingMessages, pubsub.MaxReadMessageSize) // we write the max read
	if err != nil {
		return err
	}
	defer scli.Close()
	if err := scli.RegisterBlocks(); err != nil {
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
		blk, results, prices, err := scli.ListenBlock(ctx, parser)
		if err != nil {
			return err
		}
		consumed := fees.Dimensions{}
		for _, result := range results {
			nconsumed, err := fees.Add(consumed, result.Consumed)
			if err != nil {
				return err
			}
			consumed = nconsumed
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
			window.Update(&tpsWindow, window.WindowSliceSize-consts.Uint64Len, uint64(len(blk.Txs)))
			runningDuration := time.Since(start)
			tpsDivisor := min(window.WindowSize, runningDuration.Seconds())
			utils.Outf(
				"{{green}}height:{{/}}%d {{green}}txs:{{/}}%d {{green}}root:{{/}}%s {{green}}size:{{/}}%.2fKB {{green}}units consumed:{{/}} [%s] {{green}}unit prices:{{/}} [%s] [{{green}}TPS:{{/}}%.2f {{green}}latency:{{/}}%dms {{green}}gap:{{/}}%dms]\n",
				blk.Hght,
				len(blk.Txs),
				blk.StateRoot,
				float64(blk.Size())/units.KiB,
				ParseDimensions(consumed),
				ParseDimensions(prices),
				float64(window.Sum(tpsWindow))/tpsDivisor,
				time.Now().UnixMilli()-blk.Tmstmp,
				time.Since(lastBlockDetailed).Milliseconds(),
			)
		} else {
			utils.Outf(
				"{{green}}height:{{/}}%d {{green}}txs:{{/}}%d {{green}}root:{{/}}%s {{green}}size:{{/}}%.2fKB {{green}}units consumed:{{/}} [%s] {{green}}unit prices:{{/}} [%s]\n",
				blk.Hght,
				len(blk.Txs),
				blk.StateRoot,
				float64(blk.Size())/units.KiB,
				ParseDimensions(consumed),
				ParseDimensions(prices),
			)
			window.Update(&tpsWindow, window.WindowSliceSize-consts.Uint64Len, uint64(len(blk.Txs)))
		}
		lastBlock = now.Unix()
		lastBlockDetailed = now
		if hideTxs {
			continue
		}
		for i, tx := range blk.Txs {
			handleTx(tx, results[i])
		}
	}
	return nil
}
