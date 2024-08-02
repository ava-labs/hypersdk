// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cli

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	runner "github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/window"
	"golang.org/x/exp/maps"
	"gopkg.in/yaml.v2"
)

func (h *Handler) ImportChain() error {
	chainID, err := h.PromptID("chainID")
	if err != nil {
		return err
	}
	name, err := h.PromptString("name", 0, consts.MaxInt)
	if err != nil {
		return err
	}
	uri, err := h.PromptString("uri", 0, consts.MaxInt)
	if err != nil {
		return err
	}
	if err := h.StoreChain(chainID, name, uri); err != nil {
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
		name := nodeInfo.GetName()
		trackedSubnets := strings.Split(nodeInfo.WhitelistedSubnets, ",")
		for _, subnet := range trackedSubnets {
			subnetID, err := ids.FromString(subnet)
			if err != nil {
				return err
			}
			for _, chainID := range subnets[subnetID] {
				uri := fmt.Sprintf("%s/ext/bc/%s", nodeInfo.Uri, chainID)
				if err := h.StoreChain(chainID, name, uri); err != nil {
					return err
				}
				utils.Outf(
					"{{yellow}}[%s] stored chainID:{{/}} %s {{yellow}}uri:{{/}} %s\n",
					name,
					chainID,
					uri,
				)
				filledChainID = chainID
			}
		}
	}
	return h.StoreDefaultChain(filledChainID)
}

type ClusterInfo struct {
	ChainID  string `yaml:"CHAIN_ID"` // ids.ID requires "first and last characters to be quotes"
	SubnetID string `yaml:"SUBNET_ID"`
	APIs     []struct {
		CloudID string `yaml:"CLOUD_ID"`
		IP      string `yaml:"IP"`
		Region  string `yaml:"REGION"`
	} `yaml:"API"`
	Validators []struct {
		CloudID string `yaml:"CLOUD_ID"`
		IP      string `yaml:"IP"`
		Region  string `yaml:"REGION"`
		NodeID  string `yaml:"NODE_ID"`
	} `yaml:"VALIDATOR"`
}

func ReadCLIFile(cliPath string) (ids.ID, map[string]string, error) {
	// Load yaml file
	yamlFile, err := os.ReadFile(cliPath)
	if err != nil {
		return ids.Empty, nil, err
	}
	var yamlContents ClusterInfo
	if err := yaml.Unmarshal(yamlFile, &yamlContents); err != nil {
		return ids.Empty, nil, fmt.Errorf("%w: unable to unmarshal YAML", err)
	}
	chainID, err := ids.FromString(yamlContents.ChainID)
	if err != nil {
		return ids.Empty, nil, err
	}

	// Load nodes
	nodes := make(map[string]string)
	for i, api := range yamlContents.APIs {
		name := fmt.Sprintf("%s-%d (%s)", "API", i, api.Region)
		uri := fmt.Sprintf("http://%s:9650/ext/bc/%s", api.IP, chainID)
		nodes[name] = uri
	}
	for i, validator := range yamlContents.Validators {
		name := fmt.Sprintf("%s-%d (%s)", "Validator", i, validator.Region)
		uri := fmt.Sprintf("http://%s:9650/ext/bc/%s", validator.IP, chainID)
		nodes[name] = uri
	}
	return chainID, nodes, nil
}

func (h *Handler) ImportCLI(cliPath string) error {
	oldChains, err := h.DeleteChains()
	if err != nil {
		return err
	}
	if len(oldChains) > 0 {
		utils.Outf("{{yellow}}deleted old chains:{{/}} %+v\n", oldChains)
	}

	// Load yaml file
	chainID, nodes, err := ReadCLIFile(cliPath)
	if err != nil {
		return err
	}
	for name, uri := range nodes {
		if err := h.StoreChain(chainID, name, uri); err != nil {
			return err
		}
		utils.Outf(
			"{{yellow}}[%s] stored chainID:{{/}} %s {{yellow}}uri:{{/}} %s\n",
			name,
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
	uriName := maps.Keys(uris)[0]
	cli := rpc.NewJSONRPCClient(uris[uriName])
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
	uriName := onlyAPIs(uris)[0]
	utils.Outf("{{yellow}}uri:{{/}} %s\n", uris[uriName])
	rcli := rpc.NewJSONRPCClient(uris[uriName])
	networkID, _, _, err := rcli.Network(context.TODO())
	if err != nil {
		return err
	}
	parser, err := getParser(uris[uriName], networkID, chainID)
	if err != nil {
		return err
	}
	scli, err := rpc.NewWebSocketClient(
		uris[uriName],
		rpc.DefaultHandshakeTimeout,
		pubsub.MaxPendingMessages,
		consts.MTU,
		pubsub.MaxReadMessageSize,
	) // we write the max read
	if err != nil {
		return err
	}
	defer scli.Close()
	if err := scli.RegisterChunks(); err != nil {
		return err
	}
	utils.Outf("{{green}}watching for new blocks on %s 👀{{/}}\n", chainID)
	var (
		start     time.Time
		lastBlock int64
		tpsWindow = window.Window{}
	)
	for ctx.Err() == nil {
		blk, chunk, results, err := scli.ListenChunk(ctx, parser)
		if err != nil {
			return fmt.Errorf("%w: chunk is invalid", err)
		}
		chunkID, err := chunk.ID()
		if err != nil {
			return err
		}
		consumed := chain.Dimensions{}
		for _, result := range results {
			nconsumed, err := chain.Add(consumed, result.Consumed)
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
			window.Update(&tpsWindow, window.WindowSliceSize-consts.Uint64Len, uint64(len(chunk.Txs)))
			runningDuration := time.Since(start)
			tpsDivisor := math.Min(window.WindowSize, runningDuration.Seconds())
			utils.Outf(
				"{{green}}block height:{{/}}%d {{green}}chunk:{{/}}%s {{green}}producer:{{/}}%s {{green}}txs:{{/}}%d {{green}}size:{{/}}%.2fKB {{green}}units consumed:{{/}} [%s] {{green}}TPS:{{/}}%.2f\n",
				blk,
				chunkID,
				chunk.Producer,
				len(chunk.Txs),
				float64(chunk.Size())/units.KiB,
				ParseDimensions(consumed),
				float64(window.Sum(tpsWindow))/tpsDivisor,
			)
		} else {
			utils.Outf(
				"{{green}}block height:{{/}}%d {{green}}chunk:{{/}}%s {{green}}producer:{{/}}%s {{green}}txs:{{/}}%d {{green}}size:{{/}}%.2fKB {{green}}units consumed:{{/}} [%s]\n",
				blk,
				chunkID,
				chunk.Producer,
				len(chunk.Txs),
				float64(chunk.Size())/units.KiB,
				ParseDimensions(consumed),
			)
			window.Update(&tpsWindow, window.WindowSliceSize-consts.Uint64Len, uint64(len(chunk.Txs)))
		}
		lastBlock = now.Unix()
		if hideTxs {
			continue
		}
		for i, tx := range chunk.Txs {
			handleTx(tx, results[i])
		}
	}
	return nil
}

func (h *Handler) WatchPreconfs() error {
	ctx := context.Background()
	chainID, uris, err := h.PromptChain("select chainID", nil)
	if err != nil {
		return err
	}
	if err := h.CloseDatabase(); err != nil {
		return err
	}
	uriName := onlyAPIs(uris)[0]
	utils.Outf("{{yellow}}uri:{{/}} %s\n", uris[uriName])
	rcli := rpc.NewJSONRPCClient(uris[uriName])
	scli, err := rpc.NewWebSocketClient(
		uris[uriName],
		rpc.DefaultHandshakeTimeout,
		pubsub.MaxPendingMessages,
		consts.MTU,
		pubsub.MaxReadMessageSize,
	) // we write the max read
	if err != nil {
		return err
	}
	defer scli.Close()
	if err := scli.RegisterPreConf(); err != nil {
		return err
	}
	utils.Outf("watching for preconfs on %s 👀{{/}}\n", chainID)

	for ctx.Err() == nil {
		chunkID, err := scli.ListenPreConf(ctx)
		if err != nil {
			return fmt.Errorf("%w: preconf unpack err", err)
		}
		_, h, _, _ := rcli.Accepted(ctx)
		utils.Outf("preconf issued for chunk id: %s{{/}} at height: %d\n", chunkID, h)
	}
	return nil
}

func (h *Handler) WatchPreconfsHonored() error {
	ctx := context.Background()
	chainID, uris, err := h.PromptChain("select chainID", nil)
	if err != nil {
		return err
	}
	if err := h.CloseDatabase(); err != nil {
		return err
	}
	uriName := onlyAPIs(uris)[0]
	utils.Outf("{{yellow}}uri:{{/}} %s\n", uris[uriName])
	scli, err := rpc.NewWebSocketClient(
		uris[uriName],
		rpc.DefaultHandshakeTimeout,
		pubsub.MaxPendingMessages,
		consts.MTU,
		pubsub.MaxReadMessageSize,
	) // we write the max read
	if err != nil {
		return err
	}
	defer scli.Close()
	if err := scli.RegisterBlocks(); err != nil {
		return err
	}
	utils.Outf("watching for preconfs on %s 👀{{/}}\n", chainID)

	for ctx.Err() == nil {
		blk, chunkIDs, err := scli.ListenBlock(ctx)
		if err != nil {
			return fmt.Errorf("%w: blk unpack err", err)
		}
		for _, chunkID := range chunkIDs {
			utils.Outf("preconf honored for chunk id: %s{{/}} at height: %d\n", chunkID, blk.Height)
		}
	}
	return nil
}
