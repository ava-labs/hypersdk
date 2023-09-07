package main

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/hypersdk/chain"
	hcli "github.com/ava-labs/hypersdk/cli"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/examples/tokenvm/cmd/token-cli/cmd"
	trpc "github.com/ava-labs/hypersdk/examples/tokenvm/rpc"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/window"

	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	databasePath = ".token-browser"
)

// App struct
type App struct {
	ctx context.Context

	log logger.Logger
	h   *hcli.Handler

	blocks  []*BlockInfo
	blocksL sync.Mutex
}

type BlockInfo struct {
	Timestamp int64
	ID        string
	Height    string
	Size      string
	TPS       string
	Consumed  string
	Prices    string
	StateRoot string
	Txs       int
	Latency   int64
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		log:    logger.NewDefaultLogger(),
		blocks: []*BlockInfo{},
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	h, err := hcli.New(cmd.NewController(databasePath))
	if err != nil {
		a.log.Error(err.Error())
		runtime.Quit(ctx)
		return
	}
	a.h = h

	// Generate key
	keys, err := h.GetKeys()
	if err != nil {
		a.log.Error(err.Error())
		runtime.Quit(ctx)
		return
	}
	if len(keys) == 0 {
		if err := h.GenerateKey(); err != nil {
			a.log.Error(err.Error())
			runtime.Quit(ctx)
			return
		}
	}

	// Import ANR
	if err := h.ImportANR(); err != nil {
		a.log.Error(err.Error())
		runtime.Quit(ctx)
		return
	}

	// Start fetching blocks
	go a.collectBlocks()
}

func (a *App) collectBlocks() {
	ctx := context.Background()
	chainID, uris, err := a.h.GetDefaultChain()
	if err != nil {
		a.log.Error(err.Error())
		runtime.Quit(ctx)
		return
	}

	uri := uris[0]
	rcli := rpc.NewJSONRPCClient(uri)
	networkID, _, _, err := rcli.Network(context.TODO())
	if err != nil {
		a.log.Error(err.Error())
		runtime.Quit(ctx)
		return
	}
	cli := trpc.NewJSONRPCClient(uri, networkID, chainID)
	parser, err := cli.Parser(context.TODO())
	if err != nil {
		a.log.Error(err.Error())
		runtime.Quit(ctx)
		return
	}
	scli, err := rpc.NewWebSocketClient(uri, rpc.DefaultHandshakeTimeout, pubsub.MaxPendingMessages, pubsub.MaxReadMessageSize) // we write the max read
	if err != nil {
		a.log.Error(err.Error())
		runtime.Quit(ctx)
		return
	}
	defer scli.Close()
	if err := scli.RegisterBlocks(); err != nil {
		a.log.Error(err.Error())
		runtime.Quit(ctx)
		return
	}
	var (
		start     time.Time
		lastBlock int64
		tpsWindow = window.Window{}
	)
	for ctx.Err() == nil {
		blk, results, prices, err := scli.ListenBlock(ctx, parser)
		if err != nil {
			a.log.Error(err.Error())
			runtime.Quit(ctx)
			return
		}
		consumed := chain.Dimensions{}
		for _, result := range results {
			nconsumed, err := chain.Add(consumed, result.Consumed)
			if err != nil {
				a.log.Error(err.Error())
				runtime.Quit(ctx)
				return
			}
			consumed = nconsumed
		}
		now := time.Now()
		if start.IsZero() {
			start = now
		}
		bi := &BlockInfo{}
		if lastBlock != 0 {
			since := now.Unix() - lastBlock
			newWindow, err := window.Roll(tpsWindow, int(since))
			if err != nil {
				a.log.Error(err.Error())
				runtime.Quit(ctx)
				return
			}
			tpsWindow = newWindow
			window.Update(&tpsWindow, window.WindowSliceSize-consts.Uint64Len, uint64(len(blk.Txs)))
			runningDuration := time.Since(start)
			tpsDivisor := math.Min(window.WindowSize, runningDuration.Seconds())
			bi.TPS = fmt.Sprintf("%.2f", float64(window.Sum(tpsWindow))/tpsDivisor)
			bi.Latency = time.Now().UnixMilli() - blk.Tmstmp
		} else {
			window.Update(&tpsWindow, window.WindowSliceSize-consts.Uint64Len, uint64(len(blk.Txs)))
			bi.TPS = "0.0"
		}
		blkID, err := blk.ID()
		if err != nil {
			a.log.Error(err.Error())
			runtime.Quit(ctx)
			return
		}
		bi.Timestamp = blk.Tmstmp
		bi.ID = blkID.String()
		bi.Height = strconv.FormatUint(blk.Hght, 10)
		bi.Size = fmt.Sprintf("%.2fKB", float64(blk.Size())/units.KiB)
		bi.Consumed = hcli.ParseDimensions(consumed)
		bi.Prices = hcli.ParseDimensions(prices)
		bi.StateRoot = blk.StateRoot.String()
		bi.Txs = len(blk.Txs)

		// TODO: find a more efficient way to support this
		a.blocksL.Lock()
		a.blocks = append([]*BlockInfo{bi}, a.blocks...)
		if len(a.blocks) > 100 {
			a.blocks = a.blocks[:100]
		}
		a.blocksL.Unlock()

		lastBlock = now.Unix()
	}
}

// shutdown is called after the frontend is destroyed.
func (a *App) shutdown(ctx context.Context) {
	if err := a.h.CloseDatabase(); err != nil {
		a.log.Error(err.Error())
	}
}

func (a *App) GetLatestBlocks() []*BlockInfo {
	a.blocksL.Lock()
	defer a.blocksL.Unlock()

	return a.blocks
}

func (a *App) GetChainID() string {
	chainID, _, err := a.h.GetDefaultChain()
	if err != nil {
		a.log.Error(err.Error())
		runtime.Quit(context.Background())
		return ""
	}
	return chainID.String()
}
