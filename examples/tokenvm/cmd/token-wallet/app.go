// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
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
	"github.com/ava-labs/hypersdk/examples/tokenvm/challenge"
	"github.com/ava-labs/hypersdk/examples/tokenvm/cmd/token-cli/cmd"
	frpc "github.com/ava-labs/hypersdk/examples/tokenvm/cmd/token-faucet/rpc"
	tconsts "github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	trpc "github.com/ava-labs/hypersdk/examples/tokenvm/rpc"
	"github.com/ava-labs/hypersdk/examples/tokenvm/utils"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/rpc"
	hutils "github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/window"
	"golang.org/x/exp/slices"

	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	databasePath = ".token-wallet"
	searchCores  = 4 // TODO: expose to UI
)

type Alert struct {
	Type    string
	Content string
}

type AddressInfo struct {
	Name    string
	Address string
	AddrStr string
}

// App struct
type App struct {
	ctx context.Context

	log logger.Logger
	h   *hcli.Handler

	scli *rpc.WebSocketClient
	fcli *frpc.JSONRPCClient

	workLock    sync.Mutex
	blocks      []*BlockInfo
	stats       []*TimeStat
	currentStat *TimeStat

	// TODO: move this to DB
	ownedAssets       []ids.ID
	otherAssets       []ids.ID
	transactions      []*TransactionInfo
	transactionAlerts []*Alert
	transactionLock   sync.Mutex

	searchLock   sync.Mutex
	search       *FaucetSearchInfo
	solutions    []*FaucetSearchInfo
	searchAlerts []*Alert

	addressLock sync.Mutex
	addressBook []*AddressInfo
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
		log:               logger.NewDefaultLogger(),
		blocks:            []*BlockInfo{},
		stats:             []*TimeStat{},
		transactions:      []*TransactionInfo{},
		transactionAlerts: []*Alert{},
		solutions:         []*FaucetSearchInfo{},
		searchAlerts:      []*Alert{},
		addressBook:       []*AddressInfo{},
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
	defaultKey, err := h.GetDefaultKey()
	if err != nil {
		a.log.Error(err.Error())
		runtime.Quit(ctx)
		return
	}
	a.AddAddressBook("Me", utils.Address(defaultKey.PublicKey()))

	// Import ANR
	//
	// TODO: replace with DEVENT URL
	if err := h.ImportANR(); err != nil {
		a.log.Error(err.Error())
		runtime.Quit(ctx)
		return
	}

	// Connect to Faucet
	//
	// TODO: replace with long-lived
	a.fcli = frpc.NewJSONRPCClient("http://localhost:9091")

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
			nconsumed, err := chain.Add(consumed, result.Consumed)
			if err != nil {
				a.log.Error(err.Error())
				runtime.Quit(ctx)
				return
			}
			consumed = nconsumed

			if !result.Success {
				continue
			}
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
				if action.To == pk {
					txInfo.Created = false
					a.transactions = append([]*TransactionInfo{txInfo}, a.transactions...)
					if actor != pk {
						a.transactionLock.Lock()
						a.transactionAlerts = append(a.transactionAlerts, &Alert{"info", fmt.Sprintf("Received %s %s", hutils.FormatBalance(action.Value, decimals), symbol)})
						a.transactionLock.Unlock()
					}
				} else if actor == pk {
					txInfo.Created = true
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
					a.otherAssets = append(a.otherAssets, tx.ID())
				}
			case *actions.MintAsset:
				_, symbol, decimals, _, _, _, _, err := cli.Asset(context.Background(), action.Asset, true)
				if err != nil {
					a.log.Error(err.Error())
					runtime.Quit(ctx)
					return
				}
				if action.To == pk {
					if actor != pk && !slices.Contains(a.otherAssets, action.Asset) {
						a.otherAssets = append(a.otherAssets, action.Asset)
					}
					a.transactions = append([]*TransactionInfo{{
						ID:        tx.ID().String(),
						Timestamp: blk.Tmstmp,
						Actor:     utils.Address(actor),
						Created:   false, // prefer to show receive
						Type:      "Mint",
						Units:     hcli.ParseDimensions(result.Consumed),
						Fee:       fmt.Sprintf("%s %s", hutils.FormatBalance(result.Fee, tconsts.Decimals), tconsts.Symbol),
						Summary:   fmt.Sprintf("%s %s -> %s", hutils.FormatBalance(action.Value, decimals), symbol, utils.Address(action.To)),
					}}, a.transactions...)
					if actor != pk {
						a.transactionLock.Lock()
						a.transactionAlerts = append(a.transactionAlerts, &Alert{"info", fmt.Sprintf("Received %s %s", hutils.FormatBalance(action.Value, decimals), symbol)})
						a.transactionLock.Unlock()
					}
				} else if actor == pk {
					a.transactions = append([]*TransactionInfo{{
						ID:        tx.ID().String(),
						Timestamp: blk.Tmstmp,
						Actor:     utils.Address(actor),
						Created:   true,
						Type:      "Mint",
						Units:     hcli.ParseDimensions(result.Consumed),
						Fee:       fmt.Sprintf("%s %s", hutils.FormatBalance(result.Fee, tconsts.Decimals), tconsts.Symbol),
						Summary:   fmt.Sprintf("%s %s -> %s", hutils.FormatBalance(action.Value, decimals), symbol, utils.Address(action.To)),
					}}, a.transactions...)
				}
			}
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
				// TODO: just do one loop
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
	Supply   string
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
			Supply:   hutils.FormatBalance(supply, decimals),
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

func (a *App) MintAsset(asset string, address string, amount string) error {
	ctx := context.Background()
	// TODO: share client
	_, priv, factory, cli, tcli, err := a.defaultActor()
	if err != nil {
		return err
	}

	// Input validation
	assetID, err := ids.FromString(asset)
	if err != nil {
		return err
	}
	_, _, decimals, _, _, _, _, err := tcli.Asset(context.Background(), assetID, true)
	if err != nil {
		return err
	}
	value, err := hutils.ParseBalance(amount, decimals)
	if err != nil {
		return err
	}
	to, err := utils.ParseAddress(address)
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
	_, tx, maxFee, err := cli.GenerateTransaction(ctx, parser, nil, &actions.MintAsset{
		To:    to,
		Asset: assetID,
		Value: value,
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

func (a *App) Transfer(asset string, address string, amount string) error {
	ctx := context.Background()
	// TODO: share client
	_, priv, factory, cli, tcli, err := a.defaultActor()
	if err != nil {
		return err
	}

	// Input validation
	assetID, err := ids.FromString(asset)
	if err != nil {
		return err
	}
	_, _, decimals, _, _, _, _, err := tcli.Asset(context.Background(), assetID, true)
	if err != nil {
		return err
	}
	value, err := hutils.ParseBalance(amount, decimals)
	if err != nil {
		return err
	}
	to, err := utils.ParseAddress(address)
	if err != nil {
		return err
	}

	// Ensure have sufficient balance for transfer
	sendBal, err := tcli.Balance(context.Background(), utils.Address(priv.PublicKey()), assetID)
	if err != nil {
		return err
	}
	if value > sendBal {
		return errors.New("insufficient balance")
	}

	// Ensure have sufficient balance for fees
	bal, err := tcli.Balance(context.Background(), utils.Address(priv.PublicKey()), ids.Empty)
	if err != nil {
		return err
	}

	// Generate transaction
	parser, err := tcli.Parser(ctx)
	if err != nil {
		return err
	}
	_, tx, maxFee, err := cli.GenerateTransaction(ctx, parser, nil, &actions.Transfer{
		To:    to,
		Asset: assetID,
		Value: value,
	}, factory)
	if err != nil {
		return fmt.Errorf("%w: unable to generate transaction", err)
	}
	if assetID != ids.Empty {
		if maxFee > bal {
			return errors.New("insufficient balance")
		}
	} else {
		if maxFee+value > bal {
			return errors.New("insufficient balance")
		}
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

type BalanceInfo struct {
	ID string

	Str string
	Bal string
}

func (a *App) GetBalance() ([]*BalanceInfo, error) {
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
	balances := []*BalanceInfo{{ID: ids.Empty.String(), Str: fmt.Sprintf("%s %s", hutils.FormatBalance(bal, decimals), symbol), Bal: fmt.Sprintf("%s (Balance: %s)", symbol, hutils.FormatBalance(bal, decimals))}}
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
			strAsset := asset.String()
			balances = append(balances, &BalanceInfo{ID: asset.String(), Str: fmt.Sprintf("%s %s [%s]", hutils.FormatBalance(bal, decimals), symbol, asset), Bal: fmt.Sprintf("%s [%s..%s] (Balance: %s)", symbol, strAsset[:3], strAsset[len(strAsset)-3:], hutils.FormatBalance(bal, decimals))})
		}
	}
	return balances, nil
}

type Transactions struct {
	Alerts  []*Alert
	TxInfos []*TransactionInfo
}

func (a *App) GetTransactions() *Transactions {
	a.transactionLock.Lock()
	defer a.transactionLock.Unlock()

	var alerts []*Alert
	if len(a.transactionAlerts) > 0 {
		alerts = a.transactionAlerts
		a.transactionAlerts = []*Alert{}
	}
	return &Transactions{alerts, a.transactions}
}

type FaucetSearchInfo struct {
	FaucetAddress string
	Salt          string
	Difficulty    uint16

	Solution string
	Attempts uint64
	Elapsed  string

	Amount string
	TxID   string

	Err string
}

func (a *App) StartFaucetSearch() (*FaucetSearchInfo, error) {
	priv, err := a.h.GetDefaultKey()
	if err != nil {
		return nil, err
	}

	a.searchLock.Lock()
	if a.search != nil {
		a.searchLock.Unlock()
		return nil, errors.New("already searching")
	}
	a.search = &FaucetSearchInfo{}
	a.searchLock.Unlock()

	ctx := context.Background()
	address, err := a.fcli.FaucetAddress(ctx)
	if err != nil {
		a.searchLock.Lock()
		a.search = nil
		a.searchLock.Unlock()
		return nil, err
	}
	salt, difficulty, err := a.fcli.Challenge(ctx)
	if err != nil {
		a.searchLock.Lock()
		a.search = nil
		a.searchLock.Unlock()
		return nil, err
	}
	a.search.FaucetAddress = address
	a.search.Salt = hex.EncodeToString(salt)
	a.search.Difficulty = difficulty

	// Search in the background
	go func() {
		start := time.Now()
		solution, attempts := challenge.Search(salt, difficulty, searchCores)
		txID, amount, err := a.fcli.SolveChallenge(ctx, utils.Address(priv.PublicKey()), salt, solution)
		a.searchLock.Lock()
		a.search.Solution = hex.EncodeToString(solution)
		a.search.Attempts = attempts
		a.search.Elapsed = time.Since(start).String()
		if err == nil {
			a.search.TxID = txID.String()
			a.search.Amount = fmt.Sprintf("%s %s", hutils.FormatBalance(amount, tconsts.Decimals), tconsts.Symbol)
			a.searchAlerts = append(a.searchAlerts, &Alert{"success", fmt.Sprintf("Search Successful [Attempts: %d, Elapsed: %s]", attempts, a.search.Elapsed)})
		} else {
			a.search.Err = err.Error()
			a.searchAlerts = append(a.searchAlerts, &Alert{"error", fmt.Sprintf("Search Failed: %v", err)})
		}
		search := a.search
		a.search = nil
		a.solutions = append([]*FaucetSearchInfo{search}, a.solutions...)
		a.searchLock.Unlock()
	}()
	return a.search, nil
}

// Can only return a single object
type FaucetSolutions struct {
	Alerts        []*Alert
	CurrentSearch *FaucetSearchInfo
	PastSearches  []*FaucetSearchInfo
}

func (a *App) GetFaucetSolutions() *FaucetSolutions {
	a.searchLock.Lock()
	defer a.searchLock.Unlock()

	var alerts []*Alert
	if len(a.searchAlerts) > 0 {
		alerts = a.searchAlerts
		a.searchAlerts = []*Alert{}
	}

	return &FaucetSolutions{alerts, a.search, a.solutions}
}

func (a *App) GetAddressBook() []*AddressInfo {
	a.addressLock.Lock()
	defer a.addressLock.Unlock()

	return a.addressBook
}

func (a *App) AddAddressBook(name string, address string) error {
	a.addressLock.Lock()
	defer a.addressLock.Unlock()

	name = strings.TrimSpace(name)
	address = strings.TrimSpace(address)

	// Ensure no existing addresses that match
	for _, addr := range a.addressBook {
		if addr.Address == address {
			return fmt.Errorf("duplicate address (already used for %s)", addr.Name)
		}
	}

	a.addressBook = append(a.addressBook, &AddressInfo{name, address, fmt.Sprintf("%s [%s..%s]", name, address[:len(tconsts.HRP)+3], address[len(address)-3:])})
	return nil
}
