// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/hypersdk/chain"
	hcli "github.com/ava-labs/hypersdk/cli"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/cmd/token-cli/cmd"
	tconsts "github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	trpc "github.com/ava-labs/hypersdk/examples/tokenvm/rpc"
	"github.com/ava-labs/hypersdk/examples/tokenvm/utils"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/rpc"
	hutils "github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/window"

	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	databasePath = ".token-wallet"
)

// App struct
type App struct {
	ctx context.Context

	log logger.Logger
	h   *hcli.Handler

	scli *rpc.WebSocketClient

	workLock    sync.Mutex
	blocks      []*BlockInfo
	stats       []*TimeStat
	currentStat *TimeStat

	// TODO: move this to DB
	ownedAssets  []ids.ID
	otherAssets  []ids.ID
	transactions []*TransactionInfo
}

type TransactionInfo struct {
	ID        string
	Timestamp int64
	Actor     string
	Created   bool

	Type    string
	Units   string
	Fee     string
	Summary string
}

type TimeStat struct {
	Timestamp    int64
	Transactions int
	Accounts     set.Set[string]
	Prices       chain.Dimensions
}

type BlockInfo struct {
	Timestamp int64
	ID        string
	Height    uint64
	Size      string
	TPS       string
	Consumed  string
	Prices    string
	StateRoot string

	Txs     int
	FailTxs int

	Latency int64
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		log:          logger.NewDefaultLogger(),
		blocks:       []*BlockInfo{},
		stats:        []*TimeStat{},
		transactions: []*TransactionInfo{},
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
	//
	// TODO: replace with DEVENT URL
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
	priv, err := a.h.GetDefaultKey()
	if err != nil {
		a.log.Error(err.Error())
		runtime.Quit(ctx)
		return
	}
	pk := priv.PublicKey()
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
	a.scli = scli
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
		for i, result := range results {
			tx := blk.Txs[i]
			actor := auth.GetActor(tx.Auth)
			switch action := tx.Action.(type) {
			case *actions.Transfer:
				_, symbol, decimals, _, _, _, _, err := cli.Asset(context.Background(), action.Asset, true)
				if err != nil {
					a.log.Error(err.Error())
					runtime.Quit(ctx)
					return
				}
				txInfo := &TransactionInfo{
					ID:        tx.ID().String(),
					Timestamp: blk.Tmstmp,
					Actor:     utils.Address(actor),
					Type:      "Transfer",
					Units:     hcli.ParseDimensions(result.Consumed),
					Fee:       fmt.Sprintf("%s %s", hutils.FormatBalance(result.Fee, tconsts.Decimals), tconsts.Symbol),
					Summary:   fmt.Sprintf("%s %s -> %s", hutils.FormatBalance(action.Value, decimals), symbol, utils.Address(action.To)),
				}
				if actor == pk {
					txInfo.Created = true
					a.transactions = append([]*TransactionInfo{txInfo}, a.transactions...)
				} else if action.To == pk {
					txInfo.Created = false
					a.transactions = append([]*TransactionInfo{txInfo}, a.transactions...)
				}
			case *actions.CreateAsset:
				if actor == pk {
					a.ownedAssets = append(a.ownedAssets, tx.ID())
					a.transactions = append([]*TransactionInfo{{
						ID:        tx.ID().String(),
						Timestamp: blk.Tmstmp,
						Actor:     utils.Address(actor),
						Created:   true,
						Type:      "Create",
						Units:     hcli.ParseDimensions(result.Consumed),
						Fee:       fmt.Sprintf("%s %s", hutils.FormatBalance(result.Fee, tconsts.Decimals), tconsts.Symbol),
						Summary:   fmt.Sprintf("assetID: %s symbol: %s decimals: %d metadata: %s", tx.ID(), action.Symbol, action.Decimals, action.Metadata),
					}}, a.transactions...)
				} else {
					a.otherAssets = append(a.ownedAssets, tx.ID())
				}
			}
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
		bi.Height = blk.Hght
		bi.Size = fmt.Sprintf("%.2fKB", float64(blk.Size())/units.KiB)
		bi.Consumed = hcli.ParseDimensions(consumed)
		bi.Prices = hcli.ParseDimensions(prices)
		bi.StateRoot = blk.StateRoot.String()
		for _, result := range results {
			if !result.Success {
				bi.FailTxs++
			}
		}
		bi.Txs = len(blk.Txs)

		// TODO: find a more efficient way to support this
		a.workLock.Lock()
		a.blocks = append([]*BlockInfo{bi}, a.blocks...)
		if len(a.blocks) > 100 {
			a.blocks = a.blocks[:100]
		}
		sTime := blk.Tmstmp / consts.MillisecondsPerSecond
		if a.currentStat != nil && a.currentStat.Timestamp != sTime {
			a.stats = append(a.stats, a.currentStat)
			a.currentStat = nil
		}
		if a.currentStat == nil {
			a.currentStat = &TimeStat{Timestamp: sTime, Accounts: set.Set[string]{}}
		}
		a.currentStat.Transactions += bi.Txs
		for _, tx := range blk.Txs {
			a.currentStat.Accounts.Add(string(tx.Auth.Payer()))
		}
		a.currentStat.Prices = prices
		snow := time.Now().Unix()
		newStart := 0
		for i, item := range a.stats {
			newStart = i
			if snow-item.Timestamp < 120 {
				break
			}
		}
		a.stats = a.stats[newStart:]
		a.workLock.Unlock()

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
	a.workLock.Lock()
	defer a.workLock.Unlock()

	return a.blocks
}

type GenericInfo struct {
	Timestamp int64
	Count     uint64
	Category  string
}

func (a *App) GetTransactionStats() []*GenericInfo {
	a.workLock.Lock()
	defer a.workLock.Unlock()

	info := make([]*GenericInfo, len(a.stats))
	for i := 0; i < len(a.stats); i++ {
		info[i] = &GenericInfo{a.stats[i].Timestamp, uint64(a.stats[i].Transactions), ""}
	}
	return info
}

func (a *App) GetAccountStats() []*GenericInfo {
	a.workLock.Lock()
	defer a.workLock.Unlock()

	info := make([]*GenericInfo, len(a.stats))
	for i := 0; i < len(a.stats); i++ {
		info[i] = &GenericInfo{a.stats[i].Timestamp, uint64(a.stats[i].Accounts.Len()), ""}
	}
	return info
}

func (a *App) GetUnitPrices() []*GenericInfo {
	a.workLock.Lock()
	defer a.workLock.Unlock()

	info := make([]*GenericInfo, 0, len(a.stats)*chain.FeeDimensions)
	for i := 0; i < len(a.stats); i++ {
		info = append(info, &GenericInfo{a.stats[i].Timestamp, a.stats[i].Prices[0], "Bandwidth"})
		info = append(info, &GenericInfo{a.stats[i].Timestamp, a.stats[i].Prices[1], "Compute"})
		info = append(info, &GenericInfo{a.stats[i].Timestamp, a.stats[i].Prices[2], "Storage [Read]"})
		info = append(info, &GenericInfo{a.stats[i].Timestamp, a.stats[i].Prices[3], "Storage [Create]"})
		info = append(info, &GenericInfo{a.stats[i].Timestamp, a.stats[i].Prices[4], "Storage [Modify]"})
	}
	return info
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

type AssetInfo struct {
	ID string

	Symbol   string
	Decimals int
	Metadata string
	Supply   uint64
	Creator  string
}

func (a *App) GetMyAssets() []*AssetInfo {
	_, _, _, _, tcli, err := a.defaultActor()
	if err != nil {
		a.log.Error(err.Error())
		runtime.Quit(context.Background())
		return nil
	}
	assets := []*AssetInfo{}
	for _, asset := range a.ownedAssets {
		_, symbol, decimals, metadata, supply, owner, _, err := tcli.Asset(context.Background(), asset, false)
		if err != nil {
			a.log.Error(err.Error())
			runtime.Quit(context.Background())
			return nil
		}
		assets = append(assets, &AssetInfo{
			ID:       asset.String(),
			Symbol:   string(symbol),
			Decimals: int(decimals),
			Metadata: string(metadata),
			Supply:   supply,
			Creator:  owner,
		})
	}
	return assets
}

func (a *App) defaultActor() (
	ids.ID, ed25519.PrivateKey, *auth.ED25519Factory,
	*rpc.JSONRPCClient, *trpc.JSONRPCClient, error,
) {
	priv, err := a.h.GetDefaultKey()
	if err != nil {
		return ids.Empty, ed25519.EmptyPrivateKey, nil, nil, nil, err
	}
	chainID, uris, err := a.h.GetDefaultChain()
	if err != nil {
		return ids.Empty, ed25519.EmptyPrivateKey, nil, nil, nil, err
	}
	cli := rpc.NewJSONRPCClient(uris[0])
	networkID, _, _, err := cli.Network(context.TODO())
	if err != nil {
		return ids.Empty, ed25519.EmptyPrivateKey, nil, nil, nil, err
	}
	// For [defaultActor], we always send requests to the first returned URI.
	return chainID, priv, auth.NewED25519Factory(
			priv,
		), cli,
		trpc.NewJSONRPCClient(
			uris[0],
			networkID,
			chainID,
		), nil
}

func (a *App) CreateAsset(symbol string, decimals string, metadata string) error {
	ctx := context.Background()
	// TODO: share client
	_, priv, factory, cli, tcli, err := a.defaultActor()
	if err != nil {
		return err
	}

	// Ensure have sufficient balance
	bal, err := tcli.Balance(context.Background(), utils.Address(priv.PublicKey()), ids.Empty)
	if err != nil {
		return err
	}

	// Generate transaction
	parser, err := tcli.Parser(ctx)
	if err != nil {
		return err
	}
	udecimals, err := strconv.ParseUint(decimals, 10, 8)
	if err != nil {
		return err
	}
	_, tx, maxFee, err := cli.GenerateTransaction(ctx, parser, nil, &actions.CreateAsset{
		Symbol:   []byte(symbol),
		Decimals: uint8(udecimals),
		Metadata: []byte(metadata),
	}, factory)
	if err != nil {
		return fmt.Errorf("%w: unable to generate transaction", err)
	}
	if maxFee > bal {
		return errors.New("insufficient balance")
	}
	if err := a.scli.RegisterTx(tx); err != nil {
		return err
	}

	// Wait for transaction
	_, dErr, result, err := a.scli.ListenTx(ctx)
	if err != nil {
		return err
	}
	if dErr != nil {
		return err
	}
	if !result.Success {
		return fmt.Errorf("transaction failed on-chain: %s", result.Output)
	}
	return nil
}

func (a *App) GetAddress() (string, error) {
	_, priv, _, _, _, err := a.defaultActor()
	if err != nil {
		return "", err
	}
	return utils.Address(priv.PublicKey()), nil
}

func (a *App) GetBalance() ([]string, error) {
	_, priv, _, _, tcli, err := a.defaultActor()
	if err != nil {
		return nil, err
	}
	_, symbol, decimals, _, _, _, _, err := tcli.Asset(context.Background(), ids.Empty, true)
	if err != nil {
		return nil, err
	}
	bal, err := tcli.Balance(context.Background(), utils.Address(priv.PublicKey()), ids.Empty)
	if err != nil {
		return nil, err
	}
	balances := []string{fmt.Sprintf("%s %s", hutils.FormatBalance(bal, decimals), symbol)}
	for _, arr := range [][]ids.ID{a.ownedAssets, a.otherAssets} {
		for _, asset := range arr {
			_, symbol, decimals, _, _, _, _, err := tcli.Asset(context.Background(), asset, true)
			if err != nil {
				return nil, err
			}
			bal, err := tcli.Balance(context.Background(), utils.Address(priv.PublicKey()), asset)
			if err != nil {
				return nil, err
			}
			balances = append(balances, fmt.Sprintf("%s %s (%s)", hutils.FormatBalance(bal, decimals), symbol, asset))
		}
	}
	return balances, nil
}

func (a *App) GetTransactions() []*TransactionInfo {
	return a.transactions
}
