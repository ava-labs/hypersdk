package backend

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"path"
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
)

const (
	// TODO: load config for network and faucet
	databaseFolder = ".token-wallet"
	searchCores    = 4 // TODO: expose to UI
)

type Backend struct {
	ctx   context.Context
	fatal func(error)

	// TODO: remove this
	h *hcli.Handler

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

	orderLock sync.Mutex
	myOrders  []ids.ID
}

// NewApp creates a new App application struct
func New(fatal func(error)) *Backend {
	return &Backend{
		fatal: fatal,

		blocks:            []*BlockInfo{},
		stats:             []*TimeStat{},
		transactions:      []*TransactionInfo{},
		transactionAlerts: []*Alert{},
		solutions:         []*FaucetSearchInfo{},
		searchAlerts:      []*Alert{},
		addressBook:       []*AddressInfo{},
		myOrders:          []ids.ID{},
	}
}

func (b *Backend) Start(ctx context.Context) error {
	b.ctx = ctx
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	databasePath := path.Join(homeDir, databaseFolder)
	h, err := hcli.New(cmd.NewController(databasePath))
	if err != nil {
		return err
	}
	b.h = h

	// Generate key
	keys, err := h.GetKeys()
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		if err := h.GenerateKey(); err != nil {
			return err
		}
	}
	defaultKey, err := h.GetDefaultKey(false)
	if err != nil {
		return err
	}
	b.AddAddressBook("Me", utils.Address(defaultKey.PublicKey()))
	b.otherAssets = append(b.otherAssets, ids.Empty)

	// Import ANR
	//
	// TODO: replace with DEVENT URL
	if err := h.ImportANR(); err != nil {
		return err
	}

	// Connect to Faucet
	//
	// TODO: replace with long-lived
	b.fcli = frpc.NewJSONRPCClient("http://localhost:9091")

	// Start fetching blocks
	go b.collectBlocks()
	return nil
}

func (b *Backend) collectBlocks() {
	priv, err := b.h.GetDefaultKey(true)
	if err != nil {
		b.fatal(err)
		return
	}
	pk := priv.PublicKey()
	chainID, uris, err := b.h.GetDefaultChain(true)
	if err != nil {
		b.fatal(err)
		return
	}

	uri := uris[0]
	rcli := rpc.NewJSONRPCClient(uri)
	networkID, _, _, err := rcli.Network(b.ctx)
	if err != nil {
		b.fatal(err)
		return
	}
	cli := trpc.NewJSONRPCClient(uri, networkID, chainID)
	parser, err := cli.Parser(b.ctx)
	if err != nil {
		b.fatal(err)
		return
	}
	scli, err := rpc.NewWebSocketClient(uri, rpc.DefaultHandshakeTimeout, pubsub.MaxPendingMessages, pubsub.MaxReadMessageSize) // we write the max read
	if err != nil {
		b.fatal(err)
		return
	}
	defer scli.Close()
	b.scli = scli
	if err := scli.RegisterBlocks(); err != nil {
		b.fatal(err)
		return
	}
	var (
		start     time.Time
		lastBlock int64
		tpsWindow = window.Window{}
	)

	for b.ctx.Err() == nil {
		blk, results, prices, err := scli.ListenBlock(b.ctx, parser)
		if err != nil {
			b.fatal(err)
			return
		}
		consumed := chain.Dimensions{}
		failTxs := 0
		for i, result := range results {
			nconsumed, err := chain.Add(consumed, result.Consumed)
			if err != nil {
				b.fatal(err)
				return
			}
			consumed = nconsumed

			tx := blk.Txs[i]
			actor := auth.GetActor(tx.Auth)
			if !result.Success {
				failTxs++
			}

			// We should exit action parsing as soon as possible
			switch action := tx.Action.(type) {
			case *actions.Transfer:
				if actor != pk && action.To != pk {
					continue
				}

				_, symbol, decimals, _, _, _, _, err := cli.Asset(b.ctx, action.Asset, true)
				if err != nil {
					b.fatal(err)
					return
				}
				txInfo := &TransactionInfo{
					ID:        tx.ID().String(),
					Size:      fmt.Sprintf("%.2fKB", float64(tx.Size())/units.KiB),
					Success:   result.Success,
					Timestamp: blk.Tmstmp,
					Actor:     utils.Address(actor),
					Type:      "Transfer",
					Units:     hcli.ParseDimensions(result.Consumed),
					Fee:       fmt.Sprintf("%s %s", hutils.FormatBalance(result.Fee, tconsts.Decimals), tconsts.Symbol),
				}
				if result.Success {
					txInfo.Summary = fmt.Sprintf("%s %s -> %s", hutils.FormatBalance(action.Value, decimals), symbol, utils.Address(action.To))
				} else {
					txInfo.Summary = string(result.Output)
				}
				if action.To == pk {
					if !slices.Contains(b.ownedAssets, action.Asset) && !slices.Contains(b.otherAssets, action.Asset) {
						b.otherAssets = append(b.otherAssets, action.Asset)
					}

					b.transactions = append([]*TransactionInfo{txInfo}, b.transactions...)
					if actor != pk && result.Success {
						b.transactionLock.Lock()
						b.transactionAlerts = append(b.transactionAlerts, &Alert{"info", fmt.Sprintf("Received %s %s from Transfer Action", hutils.FormatBalance(action.Value, decimals), symbol)})
						b.transactionLock.Unlock()
					}
				} else if actor == pk {
					b.transactions = append([]*TransactionInfo{txInfo}, b.transactions...)
				}
			case *actions.CreateAsset:
				if actor != pk {
					continue
				}

				b.ownedAssets = append(b.ownedAssets, tx.ID())
				txInfo := &TransactionInfo{
					ID:        tx.ID().String(),
					Size:      fmt.Sprintf("%.2fKB", float64(tx.Size())/units.KiB),
					Success:   result.Success,
					Timestamp: blk.Tmstmp,
					Actor:     utils.Address(actor),
					Type:      "CreateAsset",
					Units:     hcli.ParseDimensions(result.Consumed),
					Fee:       fmt.Sprintf("%s %s", hutils.FormatBalance(result.Fee, tconsts.Decimals), tconsts.Symbol),
				}
				if result.Success {
					txInfo.Summary = fmt.Sprintf("assetID: %s symbol: %s decimals: %d metadata: %s", tx.ID(), action.Symbol, action.Decimals, action.Metadata)
				} else {
					txInfo.Summary = string(result.Output)
				}
				b.transactions = append([]*TransactionInfo{txInfo}, b.transactions...)
			case *actions.MintAsset:
				if actor != pk && action.To != pk {
					continue
				}

				_, symbol, decimals, _, _, _, _, err := cli.Asset(b.ctx, action.Asset, true)
				if err != nil {
					b.fatal(err)
					return
				}
				txInfo := &TransactionInfo{
					ID:        tx.ID().String(),
					Timestamp: blk.Tmstmp,
					Size:      fmt.Sprintf("%.2fKB", float64(tx.Size())/units.KiB),
					Success:   result.Success,
					Actor:     utils.Address(actor),
					Type:      "Mint",
					Units:     hcli.ParseDimensions(result.Consumed),
					Fee:       fmt.Sprintf("%s %s", hutils.FormatBalance(result.Fee, tconsts.Decimals), tconsts.Symbol),
				}
				if result.Success {
					txInfo.Summary = fmt.Sprintf("%s %s -> %s", hutils.FormatBalance(action.Value, decimals), symbol, utils.Address(action.To))
				} else {
					txInfo.Summary = string(result.Output)
				}
				if action.To == pk {
					if !slices.Contains(b.ownedAssets, action.Asset) && !slices.Contains(b.otherAssets, action.Asset) {
						b.otherAssets = append(b.otherAssets, action.Asset)
					}

					b.transactions = append([]*TransactionInfo{txInfo}, b.transactions...)
					if actor != pk && result.Success {
						b.transactionLock.Lock()
						b.transactionAlerts = append(b.transactionAlerts, &Alert{"info", fmt.Sprintf("Received %s %s from Mint Action", hutils.FormatBalance(action.Value, decimals), symbol)})
						b.transactionLock.Unlock()
					}
				} else if actor == pk {
					b.transactions = append([]*TransactionInfo{txInfo}, b.transactions...)
				}
			case *actions.CreateOrder:
				if actor != pk {
					continue
				}

				_, inSymbol, inDecimals, _, _, _, _, err := cli.Asset(b.ctx, action.In, true)
				if err != nil {
					b.fatal(err)
					return
				}
				_, outSymbol, outDecimals, _, _, _, _, err := cli.Asset(b.ctx, action.Out, true)
				if err != nil {
					b.fatal(err)
					return
				}
				txInfo := &TransactionInfo{
					ID:        tx.ID().String(),
					Timestamp: blk.Tmstmp,
					Size:      fmt.Sprintf("%.2fKB", float64(tx.Size())/units.KiB),
					Success:   result.Success,
					Actor:     utils.Address(actor),
					Type:      "CreateOrder",
					Units:     hcli.ParseDimensions(result.Consumed),
					Fee:       fmt.Sprintf("%s %s", hutils.FormatBalance(result.Fee, tconsts.Decimals), tconsts.Symbol),
				}
				if result.Success {
					txInfo.Summary = fmt.Sprintf("%s %s -> %s %s (supply: %s %s)",
						hutils.FormatBalance(action.InTick, inDecimals),
						inSymbol,
						hutils.FormatBalance(action.OutTick, outDecimals),
						outSymbol,
						hutils.FormatBalance(action.Supply, outDecimals),
						outSymbol,
					)
				} else {
					txInfo.Summary = string(result.Output)
				}
				b.transactions = append([]*TransactionInfo{txInfo}, b.transactions...)
			case *actions.FillOrder:
				if actor != pk && action.Owner != pk {
					continue
				}

				_, inSymbol, inDecimals, _, _, _, _, err := cli.Asset(b.ctx, action.In, true)
				if err != nil {
					b.fatal(err)
					return
				}
				_, outSymbol, outDecimals, _, _, _, _, err := cli.Asset(b.ctx, action.Out, true)
				if err != nil {
					b.fatal(err)
					return
				}
				txInfo := &TransactionInfo{
					ID:        tx.ID().String(),
					Timestamp: blk.Tmstmp,
					Size:      fmt.Sprintf("%.2fKB", float64(tx.Size())/units.KiB),
					Success:   result.Success,
					Actor:     utils.Address(actor),
					Type:      "FillOrder",
					Units:     hcli.ParseDimensions(result.Consumed),
					Fee:       fmt.Sprintf("%s %s", hutils.FormatBalance(result.Fee, tconsts.Decimals), tconsts.Symbol),
				}
				if result.Success {
					or, _ := actions.UnmarshalOrderResult(result.Output)
					txInfo.Summary = fmt.Sprintf("%s %s -> %s %s (remaining: %s %s)",
						hutils.FormatBalance(or.In, inDecimals),
						inSymbol,
						hutils.FormatBalance(or.Out, outDecimals),
						outSymbol,
						hutils.FormatBalance(or.Remaining, outDecimals),
						outSymbol,
					)

					if action.Owner == pk && actor != pk {
						b.transactionLock.Lock()
						b.transactionAlerts = append(b.transactionAlerts, &Alert{"info", fmt.Sprintf("Received %s %s from FillOrder Action", hutils.FormatBalance(or.In, inDecimals), inSymbol)})
						b.transactionLock.Unlock()
					}
				} else {
					txInfo.Summary = string(result.Output)
				}
				if actor == pk {
					b.transactions = append([]*TransactionInfo{txInfo}, b.transactions...)
				}
			case *actions.CloseOrder:
				if actor != pk {
					continue
				}

				txInfo := &TransactionInfo{
					ID:        tx.ID().String(),
					Timestamp: blk.Tmstmp,
					Size:      fmt.Sprintf("%.2fKB", float64(tx.Size())/units.KiB),
					Success:   result.Success,
					Actor:     utils.Address(actor),
					Type:      "CloseOrder",
					Units:     hcli.ParseDimensions(result.Consumed),
					Fee:       fmt.Sprintf("%s %s", hutils.FormatBalance(result.Fee, tconsts.Decimals), tconsts.Symbol),
				}
				if result.Success {
					txInfo.Summary = fmt.Sprintf("OrderID: %s", action.Order)
				} else {
					txInfo.Summary = string(result.Output)
				}
				b.transactions = append([]*TransactionInfo{txInfo}, b.transactions...)
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
				b.fatal(err)
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
			b.fatal(err)
			return
		}
		bi.Timestamp = blk.Tmstmp
		bi.ID = blkID.String()
		bi.Height = blk.Hght
		bi.Size = fmt.Sprintf("%.2fKB", float64(blk.Size())/units.KiB)
		bi.Consumed = hcli.ParseDimensions(consumed)
		bi.Prices = hcli.ParseDimensions(prices)
		bi.StateRoot = blk.StateRoot.String()
		bi.FailTxs = failTxs
		bi.Txs = len(blk.Txs)

		// TODO: find a more efficient way to support this
		b.workLock.Lock()
		b.blocks = append([]*BlockInfo{bi}, b.blocks...)
		if len(b.blocks) > 100 {
			b.blocks = b.blocks[:100]
		}
		sTime := blk.Tmstmp / consts.MillisecondsPerSecond
		if b.currentStat != nil && b.currentStat.Timestamp != sTime {
			b.stats = append(b.stats, b.currentStat)
			b.currentStat = nil
		}
		if b.currentStat == nil {
			b.currentStat = &TimeStat{Timestamp: sTime, Accounts: set.Set[string]{}}
		}
		b.currentStat.Transactions += bi.Txs
		for _, tx := range blk.Txs {
			b.currentStat.Accounts.Add(string(tx.Auth.Payer()))
		}
		b.currentStat.Prices = prices
		snow := time.Now().Unix()
		newStart := 0
		for i, item := range b.stats {
			newStart = i
			if snow-item.Timestamp < 120 {
				break
			}
		}
		b.stats = b.stats[newStart:]
		b.workLock.Unlock()

		lastBlock = now.Unix()
	}
}

func (b *Backend) Shutdown(ctx context.Context) error {
	return b.h.CloseDatabase()
}

func (b *Backend) GetLatestBlocks() []*BlockInfo {
	b.workLock.Lock()
	defer b.workLock.Unlock()

	return b.blocks
}

func (b *Backend) GetTransactionStats() []*GenericInfo {
	b.workLock.Lock()
	defer b.workLock.Unlock()

	info := make([]*GenericInfo, len(b.stats))
	for i := 0; i < len(b.stats); i++ {
		info[i] = &GenericInfo{b.stats[i].Timestamp, uint64(b.stats[i].Transactions), ""}
	}
	return info
}

func (b *Backend) GetAccountStats() []*GenericInfo {
	b.workLock.Lock()
	defer b.workLock.Unlock()

	info := make([]*GenericInfo, len(b.stats))
	for i := 0; i < len(b.stats); i++ {
		info[i] = &GenericInfo{b.stats[i].Timestamp, uint64(b.stats[i].Accounts.Len()), ""}
	}
	return info
}

func (b *Backend) GetUnitPrices() []*GenericInfo {
	b.workLock.Lock()
	defer b.workLock.Unlock()

	info := make([]*GenericInfo, 0, len(b.stats)*chain.FeeDimensions)
	for i := 0; i < len(b.stats); i++ {
		info = append(info, &GenericInfo{b.stats[i].Timestamp, b.stats[i].Prices[0], "Bandwidth"})
		info = append(info, &GenericInfo{b.stats[i].Timestamp, b.stats[i].Prices[1], "Compute"})
		info = append(info, &GenericInfo{b.stats[i].Timestamp, b.stats[i].Prices[2], "Storage [Read]"})
		info = append(info, &GenericInfo{b.stats[i].Timestamp, b.stats[i].Prices[3], "Storage [Create]"})
		info = append(info, &GenericInfo{b.stats[i].Timestamp, b.stats[i].Prices[4], "Storage [Modify]"})
	}
	return info
}

func (b *Backend) GetChainID() string {
	chainID, _, err := b.h.GetDefaultChain(false)
	if err != nil {
		b.fatal(err)
		return ""
	}
	return chainID.String()
}

func (b *Backend) GetMyAssets() []*AssetInfo {
	_, _, _, _, tcli, err := b.defaultActor()
	if err != nil {
		b.fatal(err)
		return nil
	}
	assets := []*AssetInfo{}
	for _, asset := range b.ownedAssets {
		_, symbol, decimals, metadata, supply, owner, _, err := tcli.Asset(b.ctx, asset, false)
		if err != nil {
			b.fatal(err)
			return nil
		}
		strAsset := asset.String()
		assets = append(assets, &AssetInfo{
			ID:        asset.String(),
			Symbol:    string(symbol),
			Decimals:  int(decimals),
			Metadata:  string(metadata),
			Supply:    hutils.FormatBalance(supply, decimals),
			Creator:   owner,
			StrSymbol: fmt.Sprintf("%s [%s..%s]", symbol, strAsset[:3], strAsset[len(strAsset)-3:]),
		})
	}
	return assets
}

// TODO: share client across calls
func (b *Backend) defaultActor() (
	ids.ID, ed25519.PrivateKey, *auth.ED25519Factory,
	*rpc.JSONRPCClient, *trpc.JSONRPCClient, error,
) {
	priv, err := b.h.GetDefaultKey(false)
	if err != nil {
		return ids.Empty, ed25519.EmptyPrivateKey, nil, nil, nil, err
	}
	chainID, uris, err := b.h.GetDefaultChain(false)
	if err != nil {
		return ids.Empty, ed25519.EmptyPrivateKey, nil, nil, nil, err
	}
	cli := rpc.NewJSONRPCClient(uris[0])
	networkID, _, _, err := cli.Network(b.ctx)
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

func (b *Backend) CreateAsset(symbol string, decimals string, metadata string) error {
	// TODO: share client
	_, priv, factory, cli, tcli, err := b.defaultActor()
	if err != nil {
		return err
	}

	// Ensure have sufficient balance
	bal, err := tcli.Balance(b.ctx, utils.Address(priv.PublicKey()), ids.Empty)
	if err != nil {
		return err
	}

	// Generate transaction
	parser, err := tcli.Parser(b.ctx)
	if err != nil {
		return err
	}
	udecimals, err := strconv.ParseUint(decimals, 10, 8)
	if err != nil {
		return err
	}
	_, tx, maxFee, err := cli.GenerateTransaction(b.ctx, parser, nil, &actions.CreateAsset{
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
	if err := b.scli.RegisterTx(tx); err != nil {
		return err
	}

	// Wait for transaction
	_, dErr, result, err := b.scli.ListenTx(b.ctx)
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

func (b *Backend) MintAsset(asset string, address string, amount string) error {
	// TODO: share client
	_, priv, factory, cli, tcli, err := b.defaultActor()
	if err != nil {
		return err
	}

	// Input validation
	assetID, err := ids.FromString(asset)
	if err != nil {
		return err
	}
	_, _, decimals, _, _, _, _, err := tcli.Asset(b.ctx, assetID, true)
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
	bal, err := tcli.Balance(b.ctx, utils.Address(priv.PublicKey()), ids.Empty)
	if err != nil {
		return err
	}

	// Generate transaction
	parser, err := tcli.Parser(b.ctx)
	if err != nil {
		return err
	}
	_, tx, maxFee, err := cli.GenerateTransaction(b.ctx, parser, nil, &actions.MintAsset{
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
	if err := b.scli.RegisterTx(tx); err != nil {
		return err
	}

	// Wait for transaction
	_, dErr, result, err := b.scli.ListenTx(b.ctx)
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

func (b *Backend) Transfer(asset string, address string, amount string) error {
	// TODO: share client
	_, priv, factory, cli, tcli, err := b.defaultActor()
	if err != nil {
		return err
	}

	// Input validation
	assetID, err := ids.FromString(asset)
	if err != nil {
		return err
	}
	_, _, decimals, _, _, _, _, err := tcli.Asset(b.ctx, assetID, true)
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
	sendBal, err := tcli.Balance(b.ctx, utils.Address(priv.PublicKey()), assetID)
	if err != nil {
		return err
	}
	if value > sendBal {
		return errors.New("insufficient balance")
	}

	// Ensure have sufficient balance for fees
	bal, err := tcli.Balance(b.ctx, utils.Address(priv.PublicKey()), ids.Empty)
	if err != nil {
		return err
	}

	// Generate transaction
	parser, err := tcli.Parser(b.ctx)
	if err != nil {
		return err
	}
	_, tx, maxFee, err := cli.GenerateTransaction(b.ctx, parser, nil, &actions.Transfer{
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
	if err := b.scli.RegisterTx(tx); err != nil {
		return err
	}

	// Wait for transaction
	_, dErr, result, err := b.scli.ListenTx(b.ctx)
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

func (b *Backend) GetAddress() (string, error) {
	_, priv, _, _, _, err := b.defaultActor()
	if err != nil {
		return "", err
	}
	return utils.Address(priv.PublicKey()), nil
}

func (b *Backend) GetBalance() ([]*BalanceInfo, error) {
	_, priv, _, _, tcli, err := b.defaultActor()
	if err != nil {
		return nil, err
	}
	balances := []*BalanceInfo{}
	for _, arr := range [][]ids.ID{b.ownedAssets, b.otherAssets} {
		for _, asset := range arr {
			_, symbol, decimals, _, _, _, _, err := tcli.Asset(b.ctx, asset, true)
			if err != nil {
				return nil, err
			}
			bal, err := tcli.Balance(b.ctx, utils.Address(priv.PublicKey()), asset)
			if err != nil {
				return nil, err
			}
			strAsset := asset.String()
			if asset == ids.Empty {
				balances = append(balances, &BalanceInfo{ID: asset.String(), Str: fmt.Sprintf("%s %s", hutils.FormatBalance(bal, decimals), symbol), Bal: fmt.Sprintf("%s (Balance: %s)", symbol, hutils.FormatBalance(bal, decimals))})
			} else {
				balances = append(balances, &BalanceInfo{ID: asset.String(), Str: fmt.Sprintf("%s %s [%s]", hutils.FormatBalance(bal, decimals), symbol, asset), Bal: fmt.Sprintf("%s [%s..%s] (Balance: %s)", symbol, strAsset[:3], strAsset[len(strAsset)-3:], hutils.FormatBalance(bal, decimals))})
			}
		}
	}
	return balances, nil
}

func (b *Backend) GetTransactions() *Transactions {
	b.transactionLock.Lock()
	defer b.transactionLock.Unlock()

	var alerts []*Alert
	if len(b.transactionAlerts) > 0 {
		alerts = b.transactionAlerts
		b.transactionAlerts = []*Alert{}
	}
	return &Transactions{alerts, b.transactions}
}

func (b *Backend) StartFaucetSearch() (*FaucetSearchInfo, error) {
	priv, err := b.h.GetDefaultKey(false)
	if err != nil {
		return nil, err
	}

	b.searchLock.Lock()
	if b.search != nil {
		b.searchLock.Unlock()
		return nil, errors.New("already searching")
	}
	b.search = &FaucetSearchInfo{}
	b.searchLock.Unlock()

	address, err := b.fcli.FaucetAddress(b.ctx)
	if err != nil {
		b.searchLock.Lock()
		b.search = nil
		b.searchLock.Unlock()
		return nil, err
	}
	salt, difficulty, err := b.fcli.Challenge(b.ctx)
	if err != nil {
		b.searchLock.Lock()
		b.search = nil
		b.searchLock.Unlock()
		return nil, err
	}
	b.search.FaucetAddress = address
	b.search.Salt = hex.EncodeToString(salt)
	b.search.Difficulty = difficulty

	// Search in the background
	go func() {
		start := time.Now()
		solution, attempts := challenge.Search(salt, difficulty, searchCores)
		txID, amount, err := b.fcli.SolveChallenge(b.ctx, utils.Address(priv.PublicKey()), salt, solution)
		b.searchLock.Lock()
		b.search.Solution = hex.EncodeToString(solution)
		b.search.Attempts = attempts
		b.search.Elapsed = time.Since(start).String()
		if err == nil {
			b.search.TxID = txID.String()
			b.search.Amount = fmt.Sprintf("%s %s", hutils.FormatBalance(amount, tconsts.Decimals), tconsts.Symbol)
			b.searchAlerts = append(b.searchAlerts, &Alert{"success", fmt.Sprintf("Search Successful [Attempts: %d, Elapsed: %s]", attempts, b.search.Elapsed)})
		} else {
			b.search.Err = err.Error()
			b.searchAlerts = append(b.searchAlerts, &Alert{"error", fmt.Sprintf("Search Failed: %v", err)})
		}
		search := b.search
		b.search = nil
		b.solutions = append([]*FaucetSearchInfo{search}, b.solutions...)
		b.searchLock.Unlock()
	}()
	return b.search, nil
}

func (b *Backend) GetFaucetSolutions() *FaucetSolutions {
	b.searchLock.Lock()
	defer b.searchLock.Unlock()

	var alerts []*Alert
	if len(b.searchAlerts) > 0 {
		alerts = b.searchAlerts
		b.searchAlerts = []*Alert{}
	}

	return &FaucetSolutions{alerts, b.search, b.solutions}
}

func (b *Backend) GetAddressBook() []*AddressInfo {
	b.addressLock.Lock()
	defer b.addressLock.Unlock()

	return b.addressBook
}

func (b *Backend) AddAddressBook(name string, address string) error {
	b.addressLock.Lock()
	defer b.addressLock.Unlock()

	name = strings.TrimSpace(name)
	address = strings.TrimSpace(address)

	// Ensure no existing addresses that match
	for _, addr := range b.addressBook {
		if addr.Address == address {
			return fmt.Errorf("duplicate address (already used for %s)", addr.Name)
		}
	}

	b.addressBook = append(b.addressBook, &AddressInfo{name, address, fmt.Sprintf("%s [%s..%s]", name, address[:len(tconsts.HRP)+3], address[len(address)-3:])})
	return nil
}

func (b *Backend) GetAllAssets() []*AssetInfo {
	_, _, _, _, tcli, err := b.defaultActor()
	if err != nil {
		b.fatal(err)
		return nil
	}
	assets := []*AssetInfo{}
	for _, arr := range [][]ids.ID{b.ownedAssets, b.otherAssets} {
		for _, asset := range arr {
			_, symbol, decimals, metadata, supply, owner, _, err := tcli.Asset(b.ctx, asset, false)
			if err != nil {
				b.fatal(err)
				return nil
			}
			strAsset := asset.String()
			assets = append(assets, &AssetInfo{
				ID:        asset.String(),
				Symbol:    string(symbol),
				Decimals:  int(decimals),
				Metadata:  string(metadata),
				Supply:    hutils.FormatBalance(supply, decimals),
				Creator:   owner,
				StrSymbol: fmt.Sprintf("%s [%s..%s]", symbol, strAsset[:3], strAsset[len(strAsset)-3:]),
			})
		}
	}
	return assets
}

func (b *Backend) AddAsset(asset string) error {
	assetID, err := ids.FromString(asset)
	if err != nil {
		return err
	}
	if slices.Contains(b.ownedAssets, assetID) || slices.Contains(b.otherAssets, assetID) {
		return errors.New("asset already exists")
	}
	_, priv, _, _, tcli, err := b.defaultActor()
	if err != nil {
		return err
	}
	exists, _, _, _, _, owner, _, err := tcli.Asset(b.ctx, assetID, false)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("asset does not exist")
	}
	if owner == utils.Address(priv.PublicKey()) {
		b.ownedAssets = append(b.ownedAssets, assetID)
	} else {
		b.otherAssets = append(b.otherAssets, assetID)
	}
	return nil
}

func (b *Backend) GetMyOrders() ([]*Order, error) {
	// TODO: make locking more granular
	b.orderLock.Lock()
	defer b.orderLock.Unlock()

	_, _, _, _, tcli, err := b.defaultActor()
	if err != nil {
		return nil, err
	}
	newMyOrders := make([]ids.ID, 0, len(b.myOrders))
	orders := make([]*Order, 0, len(b.myOrders))
	for _, orderID := range b.myOrders {
		order, err := tcli.GetOrder(b.ctx, orderID)
		if err != nil {
			continue
		}
		newMyOrders = append(newMyOrders, orderID)
		inID := order.InAsset
		_, inSymbol, inDecimals, _, _, _, _, err := tcli.Asset(b.ctx, inID, true)
		if err != nil {
			return nil, err
		}
		outID := order.OutAsset
		_, outSymbol, outDecimals, _, _, _, _, err := tcli.Asset(b.ctx, outID, true)
		if err != nil {
			return nil, err
		}
		orders = append(orders, &Order{
			ID:        orderID.String(),
			InID:      inID.String(),
			InSymbol:  string(inSymbol),
			OutID:     outID.String(),
			OutSymbol: string(outSymbol),
			Price:     fmt.Sprintf("%s %s / %s %s", hutils.FormatBalance(order.InTick, inDecimals), inSymbol, hutils.FormatBalance(order.OutTick, outDecimals), outSymbol),
			InTick:    fmt.Sprintf("%s %s", hutils.FormatBalance(order.InTick, inDecimals), inSymbol),
			OutTick:   fmt.Sprintf("%s %s", hutils.FormatBalance(order.OutTick, outDecimals), outSymbol),
			Rate:      float64(order.OutTick) / float64(order.InTick),
			Remaining: fmt.Sprintf("%s %s", hutils.FormatBalance(order.Remaining, outDecimals), outSymbol),
			Owner:     order.Owner,
			MaxInput:  fmt.Sprintf("%s %s", hutils.FormatBalance((order.InTick*order.Remaining)/order.OutTick, inDecimals), inSymbol),
			InputStep: hutils.FormatBalance(order.InTick, inDecimals),
		})
	}
	b.myOrders = newMyOrders
	return orders, nil
}

func (b *Backend) GetOrders(pair string) ([]*Order, error) {
	_, _, _, _, tcli, err := b.defaultActor()
	if err != nil {
		return nil, err
	}
	rawOrders, err := tcli.Orders(b.ctx, pair)
	if err != nil {
		return nil, err
	}
	if len(rawOrders) == 0 {
		return []*Order{}, nil
	}
	assetIDs := strings.Split(pair, "-")
	in := assetIDs[0]
	inID, err := ids.FromString(in)
	if err != nil {
		return nil, err
	}
	_, inSymbol, inDecimals, _, _, _, _, err := tcli.Asset(b.ctx, inID, true)
	if err != nil {
		return nil, err
	}
	out := assetIDs[1]
	outID, err := ids.FromString(out)
	if err != nil {
		return nil, err
	}
	_, outSymbol, outDecimals, _, _, _, _, err := tcli.Asset(b.ctx, outID, true)
	if err != nil {
		return nil, err
	}

	orders := make([]*Order, len(rawOrders))
	for i := 0; i < len(rawOrders); i++ {
		order := rawOrders[i]
		orders[i] = &Order{
			ID:        order.ID.String(),
			InID:      in,
			InSymbol:  string(inSymbol),
			OutID:     out,
			OutSymbol: string(outSymbol),
			Price:     fmt.Sprintf("%s %s / %s %s", hutils.FormatBalance(order.InTick, inDecimals), inSymbol, hutils.FormatBalance(order.OutTick, outDecimals), outSymbol),
			InTick:    fmt.Sprintf("%s %s", hutils.FormatBalance(order.InTick, inDecimals), inSymbol),
			OutTick:   fmt.Sprintf("%s %s", hutils.FormatBalance(order.OutTick, outDecimals), outSymbol),
			Rate:      float64(order.OutTick) / float64(order.InTick),
			Remaining: fmt.Sprintf("%s %s", hutils.FormatBalance(order.Remaining, outDecimals), outSymbol),
			Owner:     order.Owner,
			MaxInput:  fmt.Sprintf("%s %s", hutils.FormatBalance((order.InTick*order.Remaining)/order.OutTick, inDecimals), inSymbol),
			InputStep: hutils.FormatBalance(order.InTick, inDecimals),
		}
	}
	return orders, nil
}

func (b *Backend) CreateOrder(assetIn string, inTick string, assetOut string, outTick string, supply string) error {
	// TODO: share client
	_, priv, factory, cli, tcli, err := b.defaultActor()
	if err != nil {
		return err
	}
	inID, err := ids.FromString(assetIn)
	if err != nil {
		return err
	}
	outID, err := ids.FromString(assetOut)
	if err != nil {
		return err
	}
	_, _, inDecimals, _, _, _, _, err := tcli.Asset(b.ctx, inID, true)
	if err != nil {
		return err
	}
	_, outSymbol, outDecimals, _, _, _, _, err := tcli.Asset(b.ctx, outID, true)
	if err != nil {
		return err
	}

	// Ensure have sufficient balance
	bal, err := tcli.Balance(b.ctx, utils.Address(priv.PublicKey()), ids.Empty)
	if err != nil {
		return err
	}
	outBal, err := tcli.Balance(b.ctx, utils.Address(priv.PublicKey()), outID)
	if err != nil {
		return err
	}
	iTick, err := hutils.ParseBalance(inTick, inDecimals)
	if err != nil {
		return err
	}
	oTick, err := hutils.ParseBalance(outTick, outDecimals)
	if err != nil {
		return err
	}
	oSupply, err := hutils.ParseBalance(supply, outDecimals)
	if err != nil {
		return err
	}

	// Generate transaction
	parser, err := tcli.Parser(b.ctx)
	if err != nil {
		return err
	}
	_, tx, maxFee, err := cli.GenerateTransaction(b.ctx, parser, nil, &actions.CreateOrder{
		In:      inID,
		InTick:  iTick,
		Out:     outID,
		OutTick: oTick,
		Supply:  oSupply,
	}, factory)
	if err != nil {
		return fmt.Errorf("%w: unable to generate transaction", err)
	}
	if inID == ids.Empty {
		if maxFee+oSupply > bal {
			return fmt.Errorf("insufficient balance (have: %s %s, want: %s %s)", hutils.FormatBalance(bal, tconsts.Decimals), tconsts.Symbol, hutils.FormatBalance(maxFee+oSupply, tconsts.Decimals), tconsts.Symbol)
		}
	} else {
		if maxFee > bal {
			return fmt.Errorf("insufficient balance (have: %s %s, want: %s %s)", hutils.FormatBalance(bal, tconsts.Decimals), tconsts.Symbol, hutils.FormatBalance(maxFee, tconsts.Decimals), tconsts.Symbol)
		}
		if oSupply > outBal {
			return fmt.Errorf("insufficient balance (have: %s %s, want: %s %s)", hutils.FormatBalance(outBal, outDecimals), outSymbol, hutils.FormatBalance(oSupply, outDecimals), outSymbol)
		}
	}
	if err := b.scli.RegisterTx(tx); err != nil {
		return err
	}

	// Wait for transaction
	_, dErr, result, err := b.scli.ListenTx(b.ctx)
	if err != nil {
		return err
	}
	if dErr != nil {
		return err
	}
	if !result.Success {
		return fmt.Errorf("transaction failed on-chain: %s", result.Output)
	}

	// We rely on order checking to clear backlog
	b.orderLock.Lock()
	b.myOrders = append(b.myOrders, tx.ID())
	b.orderLock.Unlock()
	return nil
}

func (b *Backend) FillOrder(orderID string, orderOwner string, assetIn string, inTick string, assetOut string, amount string) error {
	// TODO: share client
	_, priv, factory, cli, tcli, err := b.defaultActor()
	if err != nil {
		return err
	}
	oID, err := ids.FromString(orderID)
	if err != nil {
		return err
	}
	owner, err := utils.ParseAddress(orderOwner)
	if err != nil {
		return err
	}
	inID, err := ids.FromString(assetIn)
	if err != nil {
		return err
	}
	outID, err := ids.FromString(assetOut)
	if err != nil {
		return err
	}
	_, inSymbol, inDecimals, _, _, _, _, err := tcli.Asset(b.ctx, inID, true)
	if err != nil {
		return err
	}

	// Ensure have sufficient balance
	bal, err := tcli.Balance(b.ctx, utils.Address(priv.PublicKey()), ids.Empty)
	if err != nil {
		return err
	}
	inBal, err := tcli.Balance(b.ctx, utils.Address(priv.PublicKey()), inID)
	if err != nil {
		return err
	}
	iTick, err := hutils.ParseBalance(inTick, inDecimals)
	if err != nil {
		return err
	}
	inAmount, err := hutils.ParseBalance(amount, inDecimals)
	if err != nil {
		return err
	}
	if inAmount%iTick != 0 {
		return fmt.Errorf("fill amount is not aligned (must be multiple of %s %s)", inTick, inSymbol)
	}

	// Generate transaction
	parser, err := tcli.Parser(b.ctx)
	if err != nil {
		return err
	}
	_, tx, maxFee, err := cli.GenerateTransaction(b.ctx, parser, nil, &actions.FillOrder{
		Order: oID,
		Owner: owner,
		In:    inID,
		Out:   outID,
		Value: inAmount,
	}, factory)
	if err != nil {
		return fmt.Errorf("%w: unable to generate transaction", err)
	}
	if inID == ids.Empty {
		if maxFee+inAmount > bal {
			return fmt.Errorf("insufficient balance (have: %s %s, want: %s %s)", hutils.FormatBalance(bal, tconsts.Decimals), tconsts.Symbol, hutils.FormatBalance(maxFee+inAmount, tconsts.Decimals), tconsts.Symbol)
		}
	} else {
		if maxFee > bal {
			return fmt.Errorf("insufficient balance (have: %s %s, want: %s %s)", hutils.FormatBalance(bal, tconsts.Decimals), tconsts.Symbol, hutils.FormatBalance(maxFee, tconsts.Decimals), tconsts.Symbol)
		}
		if inAmount > inBal {
			return fmt.Errorf("insufficient balance (have: %s %s, want: %s %s)", hutils.FormatBalance(inBal, inDecimals), inSymbol, hutils.FormatBalance(inAmount, inDecimals), inSymbol)
		}
	}
	if err := b.scli.RegisterTx(tx); err != nil {
		return err
	}

	// Wait for transaction
	_, dErr, result, err := b.scli.ListenTx(b.ctx)
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

func (b *Backend) CloseOrder(orderID string, assetOut string) error {
	// TODO: share client
	_, priv, factory, cli, tcli, err := b.defaultActor()
	if err != nil {
		return err
	}
	oID, err := ids.FromString(orderID)
	if err != nil {
		return err
	}
	outID, err := ids.FromString(assetOut)
	if err != nil {
		return err
	}

	// Ensure have sufficient balance
	bal, err := tcli.Balance(b.ctx, utils.Address(priv.PublicKey()), ids.Empty)
	if err != nil {
		return err
	}

	// Generate transaction
	parser, err := tcli.Parser(b.ctx)
	if err != nil {
		return err
	}
	_, tx, maxFee, err := cli.GenerateTransaction(b.ctx, parser, nil, &actions.CloseOrder{
		Order: oID,
		Out:   outID,
	}, factory)
	if err != nil {
		return fmt.Errorf("%w: unable to generate transaction", err)
	}
	if maxFee > bal {
		return fmt.Errorf("insufficient balance (have: %s %s, want: %s %s)", hutils.FormatBalance(bal, tconsts.Decimals), tconsts.Symbol, hutils.FormatBalance(maxFee, tconsts.Decimals), tconsts.Symbol)
	}
	if err := b.scli.RegisterTx(tx); err != nil {
		return err
	}

	// Wait for transaction
	_, dErr, result, err := b.scli.ListenTx(b.ctx)
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
