// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cli

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/utils/units"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/cli/prompt"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/internal/window"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/utils"
)

func (h *Handler) ImportChain() error {
	uri, err := prompt.String("uri", 0, consts.MaxInt)
	if err != nil {
		return err
	}
	client := jsonrpc.NewJSONRPCClient(uri)
	_, _, chainID, err := client.Network(context.TODO())
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

func (h *Handler) SetDefaultChain() error {
	chains, err := h.GetChains()
	if err != nil {
		return err
	}
	chainID, _, err := prompt.SelectChain("set default chain", chains)
	if err != nil {
		return err
	}
	return h.StoreDefaultChain(chainID)
}

func (h *Handler) PrintChainInfo() error {
	chains, err := h.GetChains()
	if err != nil {
		return err
	}
	_, uris, err := prompt.SelectChain("select chainID", chains)
	if err != nil {
		return err
	}
	cli := jsonrpc.NewJSONRPCClient(uris[0])
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

func (h *Handler) WatchChain(hideTxs bool) error {
	ctx := context.Background()
	chains, err := h.GetChains()
	if err != nil {
		return err
	}
	chainID, uris, err := prompt.SelectChain("select chainID", chains)
	if err != nil {
		return err
	}
	if err := h.CloseDatabase(); err != nil {
		return err
	}
	utils.Outf("{{yellow}}uri:{{/}} %s\n", uris[0])
	parser := h.c.GetParser()
	scli, err := ws.NewWebSocketClient(uris[0], ws.DefaultHandshakeTimeout, pubsub.MaxPendingMessages, pubsub.MaxReadMessageSize) // we write the max read
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
			nconsumed, err := fees.Add(consumed, result.Units)
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
			newWindow := window.Roll(tpsWindow, uint64(since))
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
				consumed,
				prices,
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
				consumed,
				prices,
			)
			window.Update(&tpsWindow, window.WindowSliceSize-consts.Uint64Len, uint64(len(blk.Txs)))
		}
		lastBlock = now.Unix()
		lastBlockDetailed = now
		if hideTxs {
			continue
		}
		for i, tx := range blk.Txs {
			h.c.HandleTx(tx, results[i])
		}
	}
	return nil
}
