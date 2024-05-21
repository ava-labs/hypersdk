// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package backend

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/cache"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/hypersdk/chain"
	hcli "github.com/ava-labs/hypersdk/cli"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/challenge"
	frpc "github.com/ava-labs/hypersdk/examples/tokenvm/cmd/token-faucet/rpc"
	"github.com/ava-labs/hypersdk/examples/tokenvm/cmd/token-feed/manager"
	ferpc "github.com/ava-labs/hypersdk/examples/tokenvm/cmd/token-feed/rpc"
	tconsts "github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	trpc "github.com/ava-labs/hypersdk/examples/tokenvm/rpc"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/rpc"
	hutils "github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/window"
)

const (
	databaseFolder = ".token-wallet/db"
	configFile     = ".token-wallet/config.json"
)

type Backend struct {
	ctx   context.Context
	fatal func(error)

	s *Storage
	c *Config

	priv    ed25519.PrivateKey
	factory *auth.ED25519Factory
	addr    codec.Address
	addrStr string

	cli     *rpc.JSONRPCClient
	chainID ids.ID
	scli    *rpc.WebSocketClient
	tcli    *trpc.JSONRPCClient
	parser  chain.Parser
	fcli    *frpc.JSONRPCClient
	fecli   *ferpc.JSONRPCClient

	blockLock   sync.Mutex
	blocks      []*BlockInfo
	stats       []*TimeStat
	currentStat *TimeStat

	txAlertLock       sync.Mutex
	transactionAlerts []*Alert

	searchLock   sync.Mutex
	search       *FaucetSearchInfo
	searchAlerts []*Alert

	htmlCache *cache.LRU[string, *HTMLMeta]
	urlQueue  chan string
}

// NewApp creates a new App application struct
func New(fatal func(error)) *Backend {
	return &Backend{
		fatal: fatal,

		blocks:            []*BlockInfo{},
		stats:             []*TimeStat{},
		transactionAlerts: []*Alert{},
		searchAlerts:      []*Alert{},
		htmlCache:         &cache.LRU[string, *HTMLMeta]{Size: 128},
		urlQueue:          make(chan string, 128),
	}
}

func (b *Backend) Start(ctx context.Context) error {
	b.ctx = ctx
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// Open storage
	databasePath := path.Join(homeDir, databaseFolder)
	s, err := OpenStorage(databasePath)
	if err != nil {
		return err
	}
	b.s = s

	// Generate key
	key, err := s.GetKey()
	if err != nil {
		return err
	}
	if key == ed25519.EmptyPrivateKey {
		// TODO: encrypt key
		priv, err := ed25519.GeneratePrivateKey()
		if err != nil {
			return err
		}
		if err := s.StoreKey(priv); err != nil {
			return err
		}
		key = priv
	}
	b.priv = key
	b.factory = auth.NewED25519Factory(b.priv)
	b.addr = auth.NewED25519Address(b.priv.PublicKey())
	b.addrStr = codec.MustAddressBech32(tconsts.HRP, b.addr)
	if err := b.AddAddressBook("Me", b.addrStr); err != nil {
		return err
	}
	if err := b.s.StoreAsset(ids.Empty, false); err != nil {
		return err
	}

	// Open config
	configPath := path.Join(homeDir, configFile)
	rawConifg, err := os.ReadFile(configPath)
	if err != nil {
		// TODO: replace with DEVNET
		b.c = &Config{
			TokenRPC:    "http://54.190.240.186:9090",
			FaucetRPC:   "http://54.190.240.186:9091",
			SearchCores: 4,
			FeedRPC:     "http://54.190.240.186:9092",
		}
	} else {
		var config Config
		if err := json.Unmarshal(rawConifg, &config); err != nil {
			return err
		}
		b.c = &config
	}

	// Create clients
	b.cli = rpc.NewJSONRPCClient(b.c.TokenRPC)
	networkID, _, chainID, err := b.cli.Network(b.ctx)
	if err != nil {
		return err
	}
	b.chainID = chainID
	scli, err := rpc.NewWebSocketClient(b.c.TokenRPC, rpc.DefaultHandshakeTimeout, pubsub.MaxPendingMessages, pubsub.MaxReadMessageSize)
	if err != nil {
		return err
	}
	b.scli = scli
	b.tcli = trpc.NewJSONRPCClient(b.c.TokenRPC, networkID, chainID)
	parser, err := b.tcli.Parser(b.ctx)
	if err != nil {
		return err
	}
	b.parser = parser
	b.fcli = frpc.NewJSONRPCClient(b.c.FaucetRPC)
	b.fecli = ferpc.NewJSONRPCClient(b.c.FeedRPC)

	// Start fetching blocks
	go b.collectBlocks()
	go b.parseURLs()
	return nil
}

func (b *Backend) collectBlocks() {
	if err := b.scli.RegisterBlocks(); err != nil {
		b.fatal(err)
		return
	}

	var (
		start     time.Time
		lastBlock int64
		tpsWindow = window.Window{}
	)
	for b.ctx.Err() == nil {
		blk, results, prices, err := b.scli.ListenBlock(b.ctx, b.parser)
		if err != nil {
			b.fatal(err)
			return
		}
		consumed := fees.Dimensions{}
		failTxs := 0
		for i, result := range results {
			nconsumed, err := fees.Add(consumed, result.Consumed)
			if err != nil {
				b.fatal(err)
				return
			}
			consumed = nconsumed

			tx := blk.Txs[i]
			actor := tx.Auth.Actor()
			if !result.Success {
				failTxs++
			}

			// We should exit action parsing as soon as possible
			switch action := tx.Action.(type) {
			case *actions.Transfer:
				if actor != b.addr && action.To != b.addr {
					continue
				}

				_, symbol, decimals, _, _, owner, err := b.tcli.Asset(b.ctx, action.Asset, true)
				if err != nil {
					b.fatal(err)
					return
				}
				txInfo := &TransactionInfo{
					ID:        tx.ID().String(),
					Size:      fmt.Sprintf("%.2fKB", float64(tx.Size())/units.KiB),
					Success:   result.Success,
					Timestamp: blk.Tmstmp,
					Actor:     codec.MustAddressBech32(tconsts.HRP, actor),
					Type:      "Transfer",
					Units:     hcli.ParseDimensions(result.Consumed),
					Fee:       fmt.Sprintf("%s %s", hutils.FormatBalance(result.Fee, tconsts.Decimals), tconsts.Symbol),
				}
				if result.Success {
					txInfo.Summary = fmt.Sprintf("%s %s -> %s", hutils.FormatBalance(action.Value, decimals), symbol, codec.MustAddressBech32(tconsts.HRP, action.To))
					if len(action.Memo) > 0 {
						txInfo.Summary += fmt.Sprintf(" (memo: %s)", action.Memo)
					}
				} else {
					txInfo.Summary = string(result.Output)
				}
				if action.To == b.addr {
					if actor != b.addr && result.Success {
						b.txAlertLock.Lock()
						b.transactionAlerts = append(b.transactionAlerts, &Alert{"info", fmt.Sprintf("Received %s %s from Transfer", hutils.FormatBalance(action.Value, decimals), symbol)})
						b.txAlertLock.Unlock()
					}
					hasAsset, err := b.s.HasAsset(action.Asset)
					if err != nil {
						b.fatal(err)
						return
					}
					if !hasAsset {
						if err := b.s.StoreAsset(action.Asset, b.addrStr == owner); err != nil {
							b.fatal(err)
							return
						}
					}
					if err := b.s.StoreTransaction(txInfo); err != nil {
						b.fatal(err)
						return
					}
				} else if actor == b.addr {
					if err := b.s.StoreTransaction(txInfo); err != nil {
						b.fatal(err)
						return
					}
				}
			case *actions.CreateAsset:
				if actor != b.addr {
					continue
				}

				if err := b.s.StoreAsset(tx.ID(), true); err != nil {
					b.fatal(err)
					return
				}
				txInfo := &TransactionInfo{
					ID:        tx.ID().String(),
					Size:      fmt.Sprintf("%.2fKB", float64(tx.Size())/units.KiB),
					Success:   result.Success,
					Timestamp: blk.Tmstmp,
					Actor:     codec.MustAddressBech32(tconsts.HRP, actor),
					Type:      "CreateAsset",
					Units:     hcli.ParseDimensions(result.Consumed),
					Fee:       fmt.Sprintf("%s %s", hutils.FormatBalance(result.Fee, tconsts.Decimals), tconsts.Symbol),
				}
				if result.Success {
					txInfo.Summary = fmt.Sprintf("assetID: %s symbol: %s decimals: %d metadata: %s", tx.ID(), action.Symbol, action.Decimals, action.Metadata)
				} else {
					txInfo.Summary = string(result.Output)
				}
				if err := b.s.StoreTransaction(txInfo); err != nil {
					b.fatal(err)
					return
				}
			case *actions.MintAsset:
				if actor != b.addr && action.To != b.addr {
					continue
				}

				_, symbol, decimals, _, _, owner, err := b.tcli.Asset(b.ctx, action.Asset, true)
				if err != nil {
					b.fatal(err)
					return
				}
				txInfo := &TransactionInfo{
					ID:        tx.ID().String(),
					Timestamp: blk.Tmstmp,
					Size:      fmt.Sprintf("%.2fKB", float64(tx.Size())/units.KiB),
					Success:   result.Success,
					Actor:     codec.MustAddressBech32(tconsts.HRP, actor),
					Type:      "Mint",
					Units:     hcli.ParseDimensions(result.Consumed),
					Fee:       fmt.Sprintf("%s %s", hutils.FormatBalance(result.Fee, tconsts.Decimals), tconsts.Symbol),
				}
				if result.Success {
					txInfo.Summary = fmt.Sprintf("%s %s -> %s", hutils.FormatBalance(action.Value, decimals), symbol, codec.MustAddressBech32(tconsts.HRP, action.To))
				} else {
					txInfo.Summary = string(result.Output)
				}
				if action.To == b.addr {
					if actor != b.addr && result.Success {
						b.txAlertLock.Lock()
						b.transactionAlerts = append(b.transactionAlerts, &Alert{"info", fmt.Sprintf("Received %s %s from Mint", hutils.FormatBalance(action.Value, decimals), symbol)})
						b.txAlertLock.Unlock()
					}
					hasAsset, err := b.s.HasAsset(action.Asset)
					if err != nil {
						b.fatal(err)
						return
					}
					if !hasAsset {
						if err := b.s.StoreAsset(action.Asset, b.addrStr == owner); err != nil {
							b.fatal(err)
							return
						}
					}
					if err := b.s.StoreTransaction(txInfo); err != nil {
						b.fatal(err)
						return
					}
				} else if actor == b.addr {
					if err := b.s.StoreTransaction(txInfo); err != nil {
						b.fatal(err)
						return
					}
				}
			case *actions.CreateOrder:
				if actor != b.addr {
					continue
				}

				_, inSymbol, inDecimals, _, _, _, err := b.tcli.Asset(b.ctx, action.In, true)
				if err != nil {
					b.fatal(err)
					return
				}
				_, outSymbol, outDecimals, _, _, _, err := b.tcli.Asset(b.ctx, action.Out, true)
				if err != nil {
					b.fatal(err)
					return
				}
				txInfo := &TransactionInfo{
					ID:        tx.ID().String(),
					Timestamp: blk.Tmstmp,
					Size:      fmt.Sprintf("%.2fKB", float64(tx.Size())/units.KiB),
					Success:   result.Success,
					Actor:     codec.MustAddressBech32(tconsts.HRP, actor),
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
				if err := b.s.StoreTransaction(txInfo); err != nil {
					b.fatal(err)
					return
				}
			case *actions.FillOrder:
				if actor != b.addr && action.Owner != b.addr {
					continue
				}

				_, inSymbol, inDecimals, _, _, _, err := b.tcli.Asset(b.ctx, action.In, true)
				if err != nil {
					b.fatal(err)
					return
				}
				_, outSymbol, outDecimals, _, _, _, err := b.tcli.Asset(b.ctx, action.Out, true)
				if err != nil {
					b.fatal(err)
					return
				}
				txInfo := &TransactionInfo{
					ID:        tx.ID().String(),
					Timestamp: blk.Tmstmp,
					Size:      fmt.Sprintf("%.2fKB", float64(tx.Size())/units.KiB),
					Success:   result.Success,
					Actor:     codec.MustAddressBech32(tconsts.HRP, actor),
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

					if action.Owner == b.addr && actor != b.addr {
						b.txAlertLock.Lock()
						b.transactionAlerts = append(b.transactionAlerts, &Alert{"info", fmt.Sprintf("Received %s %s from FillOrder", hutils.FormatBalance(or.In, inDecimals), inSymbol)})
						b.txAlertLock.Unlock()
					}
				} else {
					txInfo.Summary = string(result.Output)
				}
				if actor == b.addr {
					if err := b.s.StoreTransaction(txInfo); err != nil {
						b.fatal(err)
						return
					}
				}
			case *actions.CloseOrder:
				if actor != b.addr {
					continue
				}

				txInfo := &TransactionInfo{
					ID:        tx.ID().String(),
					Timestamp: blk.Tmstmp,
					Size:      fmt.Sprintf("%.2fKB", float64(tx.Size())/units.KiB),
					Success:   result.Success,
					Actor:     codec.MustAddressBech32(tconsts.HRP, actor),
					Type:      "CloseOrder",
					Units:     hcli.ParseDimensions(result.Consumed),
					Fee:       fmt.Sprintf("%s %s", hutils.FormatBalance(result.Fee, tconsts.Decimals), tconsts.Symbol),
				}
				if result.Success {
					txInfo.Summary = fmt.Sprintf("OrderID: %s", action.Order)
				} else {
					txInfo.Summary = string(result.Output)
				}
				if err := b.s.StoreTransaction(txInfo); err != nil {
					b.fatal(err)
					return
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
				b.fatal(err)
				return
			}
			tpsWindow = newWindow
			window.Update(&tpsWindow, window.WindowSliceSize-consts.Uint64Len, uint64(len(blk.Txs)))
			runningDuration := time.Since(start)
			tpsDivisor := min(window.WindowSize, runningDuration.Seconds())
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
		b.blockLock.Lock()
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
			b.currentStat.Accounts.Add(codec.MustAddressBech32(tconsts.HRP, tx.Auth.Sponsor()))
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
		b.blockLock.Unlock()

		lastBlock = now.Unix()
	}
}

func (b *Backend) Shutdown(context.Context) error {
	_ = b.scli.Close()
	return b.s.Close()
}

func (b *Backend) GetLatestBlocks() []*BlockInfo {
	b.blockLock.Lock()
	defer b.blockLock.Unlock()

	return b.blocks
}

func (b *Backend) GetTransactionStats() []*GenericInfo {
	b.blockLock.Lock()
	defer b.blockLock.Unlock()

	info := make([]*GenericInfo, len(b.stats))
	for i := 0; i < len(b.stats); i++ {
		info[i] = &GenericInfo{b.stats[i].Timestamp, uint64(b.stats[i].Transactions), ""}
	}
	return info
}

func (b *Backend) GetAccountStats() []*GenericInfo {
	b.blockLock.Lock()
	defer b.blockLock.Unlock()

	info := make([]*GenericInfo, len(b.stats))
	for i := 0; i < len(b.stats); i++ {
		info[i] = &GenericInfo{b.stats[i].Timestamp, uint64(b.stats[i].Accounts.Len()), ""}
	}
	return info
}

func (b *Backend) GetUnitPrices() []*GenericInfo {
	b.blockLock.Lock()
	defer b.blockLock.Unlock()

	info := make([]*GenericInfo, 0, len(b.stats)*fees.FeeDimensions)
	for i := 0; i < len(b.stats); i++ {
		info = append(info, &GenericInfo{b.stats[i].Timestamp, b.stats[i].Prices[0], "Bandwidth"})
		info = append(info, &GenericInfo{b.stats[i].Timestamp, b.stats[i].Prices[1], "Compute"})
		info = append(info, &GenericInfo{b.stats[i].Timestamp, b.stats[i].Prices[2], "Storage [Read]"})
		info = append(info, &GenericInfo{b.stats[i].Timestamp, b.stats[i].Prices[3], "Storage [Allocate]"})
		info = append(info, &GenericInfo{b.stats[i].Timestamp, b.stats[i].Prices[4], "Storage [Write]"})
	}
	return info
}

func (b *Backend) GetChainID() string {
	return b.chainID.String()
}

func (b *Backend) GetMyAssets() []*AssetInfo {
	assets := []*AssetInfo{}
	assetIDs, owned, err := b.s.GetAssets()
	if err != nil {
		b.fatal(err)
		return nil
	}
	for i, asset := range assetIDs {
		if !owned[i] {
			continue
		}
		_, symbol, decimals, metadata, supply, owner, err := b.tcli.Asset(b.ctx, asset, false)
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

func (b *Backend) CreateAsset(symbol string, decimals string, metadata string) error {
	// Ensure have sufficient balance
	bal, err := b.tcli.Balance(b.ctx, b.addrStr, ids.Empty)
	if err != nil {
		return err
	}

	// Generate transaction
	udecimals, err := strconv.ParseUint(decimals, 10, 8)
	if err != nil {
		return err
	}
	_, tx, maxFee, err := b.cli.GenerateTransaction(b.ctx, b.parser, &actions.CreateAsset{
		Symbol:   []byte(symbol),
		Decimals: uint8(udecimals),
		Metadata: []byte(metadata),
	}, b.factory)
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

func (b *Backend) MintAsset(asset string, address string, amount string) error {
	// Input validation
	assetID, err := ids.FromString(asset)
	if err != nil {
		return err
	}
	_, _, decimals, _, _, _, err := b.tcli.Asset(b.ctx, assetID, true)
	if err != nil {
		return err
	}
	value, err := hutils.ParseBalance(amount, decimals)
	if err != nil {
		return err
	}
	to, err := codec.ParseAddressBech32(tconsts.HRP, address)
	if err != nil {
		return err
	}

	// Ensure have sufficient balance
	bal, err := b.tcli.Balance(b.ctx, b.addrStr, ids.Empty)
	if err != nil {
		return err
	}

	// Generate transaction
	_, tx, maxFee, err := b.cli.GenerateTransaction(b.ctx, b.parser, &actions.MintAsset{
		To:    to,
		Asset: assetID,
		Value: value,
	}, b.factory)
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

func (b *Backend) Transfer(asset string, address string, amount string, memo string) error {
	// Input validation
	assetID, err := ids.FromString(asset)
	if err != nil {
		return err
	}
	_, symbol, decimals, _, _, _, err := b.tcli.Asset(b.ctx, assetID, true)
	if err != nil {
		return err
	}
	value, err := hutils.ParseBalance(amount, decimals)
	if err != nil {
		return err
	}
	to, err := codec.ParseAddressBech32(tconsts.HRP, address)
	if err != nil {
		return err
	}

	// Ensure have sufficient balance for transfer
	sendBal, err := b.tcli.Balance(b.ctx, b.addrStr, assetID)
	if err != nil {
		return err
	}
	if value > sendBal {
		return fmt.Errorf("insufficient balance (have: %s %s, want: %s %s)", hutils.FormatBalance(sendBal, decimals), symbol, hutils.FormatBalance(value, decimals), symbol)
	}

	// Ensure have sufficient balance for fees
	bal, err := b.tcli.Balance(b.ctx, b.addrStr, ids.Empty)
	if err != nil {
		return err
	}

	// Generate transaction
	_, tx, maxFee, err := b.cli.GenerateTransaction(b.ctx, b.parser, &actions.Transfer{
		To:    to,
		Asset: assetID,
		Value: value,
		Memo:  []byte(memo),
	}, b.factory)
	if err != nil {
		return fmt.Errorf("%w: unable to generate transaction", err)
	}
	if assetID != ids.Empty {
		if maxFee > bal {
			return fmt.Errorf("insufficient balance (have: %s %s, want: %s %s)", hutils.FormatBalance(bal, tconsts.Decimals), tconsts.Symbol, hutils.FormatBalance(maxFee, tconsts.Decimals), tconsts.Symbol)
		}
	} else {
		if maxFee+value > bal {
			return fmt.Errorf("insufficient balance (have: %s %s, want: %s %s)", hutils.FormatBalance(bal, tconsts.Decimals), tconsts.Symbol, hutils.FormatBalance(maxFee+value, tconsts.Decimals), tconsts.Symbol)
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

func (b *Backend) GetAddress() string {
	return b.addrStr
}

func (b *Backend) GetBalance() ([]*BalanceInfo, error) {
	assets, _, err := b.s.GetAssets()
	if err != nil {
		return nil, err
	}
	balances := []*BalanceInfo{}
	for _, asset := range assets {
		_, symbol, decimals, _, _, _, err := b.tcli.Asset(b.ctx, asset, true)
		if err != nil {
			return nil, err
		}
		bal, err := b.tcli.Balance(b.ctx, b.addrStr, asset)
		if err != nil {
			return nil, err
		}
		strAsset := asset.String()
		if asset == ids.Empty {
			balances = append(balances, &BalanceInfo{ID: asset.String(), Str: fmt.Sprintf("%s %s", hutils.FormatBalance(bal, decimals), symbol), Bal: fmt.Sprintf("%s (Balance: %s)", symbol, hutils.FormatBalance(bal, decimals)), Has: bal > 0})
		} else {
			balances = append(balances, &BalanceInfo{ID: asset.String(), Str: fmt.Sprintf("%s %s [%s]", hutils.FormatBalance(bal, decimals), symbol, asset), Bal: fmt.Sprintf("%s [%s..%s] (Balance: %s)", symbol, strAsset[:3], strAsset[len(strAsset)-3:], hutils.FormatBalance(bal, decimals)), Has: bal > 0})
		}
	}
	return balances, nil
}

func (b *Backend) GetTransactions() *Transactions {
	b.txAlertLock.Lock()
	defer b.txAlertLock.Unlock()

	var alerts []*Alert
	if len(b.transactionAlerts) > 0 {
		alerts = b.transactionAlerts
		b.transactionAlerts = []*Alert{}
	}
	txs, err := b.s.GetTransactions()
	if err != nil {
		b.fatal(err)
		return nil
	}
	return &Transactions{alerts, txs}
}

func (b *Backend) StartFaucetSearch() (*FaucetSearchInfo, error) {
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
		solution, attempts := challenge.Search(salt, difficulty, b.c.SearchCores)
		txID, amount, err := b.fcli.SolveChallenge(b.ctx, b.addrStr, salt, solution)
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
		b.searchLock.Unlock()
		if err := b.s.StoreSolution(search); err != nil {
			b.fatal(err)
		}
	}()
	return b.search, nil
}

func (b *Backend) GetFaucetSolutions() *FaucetSolutions {
	solutions, err := b.s.GetSolutions()
	if err != nil {
		b.fatal(err)
		return nil
	}

	b.searchLock.Lock()
	defer b.searchLock.Unlock()

	var alerts []*Alert
	if len(b.searchAlerts) > 0 {
		alerts = b.searchAlerts
		b.searchAlerts = []*Alert{}
	}

	return &FaucetSolutions{alerts, b.search, solutions}
}

func (b *Backend) GetAddressBook() []*AddressInfo {
	addresses, err := b.s.GetAddresses()
	if err != nil {
		b.fatal(err)
		return nil
	}
	return addresses
}

// Any existing address will be overwritten with a new name
func (b *Backend) AddAddressBook(name string, address string) error {
	name = strings.TrimSpace(name)
	address = strings.TrimSpace(address)
	return b.s.StoreAddress(address, name)
}

func (b *Backend) GetAllAssets() []*AssetInfo {
	arr, _, err := b.s.GetAssets()
	if err != nil {
		b.fatal(err)
		return nil
	}
	assets := []*AssetInfo{}
	for _, asset := range arr {
		_, symbol, decimals, metadata, supply, owner, err := b.tcli.Asset(b.ctx, asset, false)
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

func (b *Backend) AddAsset(asset string) error {
	assetID, err := ids.FromString(asset)
	if err != nil {
		return err
	}
	hasAsset, err := b.s.HasAsset(assetID)
	if err != nil {
		return err
	}
	if hasAsset {
		return nil
	}
	exists, _, _, _, _, owner, err := b.tcli.Asset(b.ctx, assetID, true)
	if err != nil {
		return err
	}
	if !exists {
		return ErrAssetMissing
	}
	return b.s.StoreAsset(assetID, owner == b.addrStr)
}

func (b *Backend) GetMyOrders() ([]*Order, error) {
	orderIDs, orderKeys, err := b.s.GetOrders()
	if err != nil {
		return nil, err
	}
	orders := make([]*Order, 0, len(orderIDs))
	for i, orderID := range orderIDs {
		order, err := b.tcli.GetOrder(b.ctx, orderID)
		if err != nil {
			if err := b.s.DeleteDBKey(orderKeys[i]); err != nil {
				return nil, err
			}
			continue
		}
		inID := order.InAsset
		_, inSymbol, inDecimals, _, _, _, err := b.tcli.Asset(b.ctx, inID, true)
		if err != nil {
			return nil, err
		}
		outID := order.OutAsset
		_, outSymbol, outDecimals, _, _, _, err := b.tcli.Asset(b.ctx, outID, true)
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
	return orders, nil
}

func (b *Backend) GetOrders(pair string) ([]*Order, error) {
	rawOrders, err := b.tcli.Orders(b.ctx, pair)
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
	_, inSymbol, inDecimals, _, _, _, err := b.tcli.Asset(b.ctx, inID, true)
	if err != nil {
		return nil, err
	}
	out := assetIDs[1]
	outID, err := ids.FromString(out)
	if err != nil {
		return nil, err
	}
	_, outSymbol, outDecimals, _, _, _, err := b.tcli.Asset(b.ctx, outID, true)
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
	inID, err := ids.FromString(assetIn)
	if err != nil {
		return err
	}
	outID, err := ids.FromString(assetOut)
	if err != nil {
		return err
	}
	_, _, inDecimals, _, _, _, err := b.tcli.Asset(b.ctx, inID, true)
	if err != nil {
		return err
	}
	_, outSymbol, outDecimals, _, _, _, err := b.tcli.Asset(b.ctx, outID, true)
	if err != nil {
		return err
	}

	// Ensure have sufficient balance
	bal, err := b.tcli.Balance(b.ctx, b.addrStr, ids.Empty)
	if err != nil {
		return err
	}
	outBal, err := b.tcli.Balance(b.ctx, b.addrStr, outID)
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
	_, tx, maxFee, err := b.cli.GenerateTransaction(b.ctx, b.parser, &actions.CreateOrder{
		In:      inID,
		InTick:  iTick,
		Out:     outID,
		OutTick: oTick,
		Supply:  oSupply,
	}, b.factory)
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
	return b.s.StoreOrder(tx.ID())
}

func (b *Backend) FillOrder(orderID string, orderOwner string, assetIn string, inTick string, assetOut string, amount string) error {
	oID, err := ids.FromString(orderID)
	if err != nil {
		return err
	}
	owner, err := codec.ParseAddressBech32(tconsts.HRP, orderOwner)
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
	_, inSymbol, inDecimals, _, _, _, err := b.tcli.Asset(b.ctx, inID, true)
	if err != nil {
		return err
	}

	// Ensure have sufficient balance
	bal, err := b.tcli.Balance(b.ctx, b.addrStr, ids.Empty)
	if err != nil {
		return err
	}
	inBal, err := b.tcli.Balance(b.ctx, b.addrStr, inID)
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
	_, tx, maxFee, err := b.cli.GenerateTransaction(b.ctx, b.parser, &actions.FillOrder{
		Order: oID,
		Owner: owner,
		In:    inID,
		Out:   outID,
		Value: inAmount,
	}, b.factory)
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
	oID, err := ids.FromString(orderID)
	if err != nil {
		return err
	}
	outID, err := ids.FromString(assetOut)
	if err != nil {
		return err
	}

	// Ensure have sufficient balance
	bal, err := b.tcli.Balance(b.ctx, b.addrStr, ids.Empty)
	if err != nil {
		return err
	}

	// Generate transaction
	_, tx, maxFee, err := b.cli.GenerateTransaction(b.ctx, b.parser, &actions.CloseOrder{
		Order: oID,
		Out:   outID,
	}, b.factory)
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

func (b *Backend) GetFeedInfo() (*FeedInfo, error) {
	addr, fee, err := b.fecli.FeedInfo(context.TODO())
	if err != nil {
		return nil, err
	}
	return &FeedInfo{
		addr,
		fmt.Sprintf("%s %s", hutils.FormatBalance(fee, tconsts.Decimals), tconsts.Symbol),
	}, nil
}

func (b *Backend) parseURLs() {
	client := http.DefaultClient
	for {
		select {
		case u := <-b.urlQueue:
			// Protect against maliciously crafted URLs
			parsedURL, err := url.Parse(u)
			if err != nil {
				continue
			}
			if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
				continue
			}
			ip := net.ParseIP(parsedURL.Host)
			if ip != nil {
				if ip.IsPrivate() || ip.IsLoopback() {
					continue
				}
			}

			// Attempt to fetch URL contents
			ctx, cancel := context.WithTimeout(b.ctx, 30*time.Second)
			req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
			if err != nil {
				cancel()
				continue
			}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Println("unable to fetch URL", err)
				// We already put the URL in as nil in
				// our cache, so we won't refetch it.
				cancel()
				continue
			}
			b.htmlCache.Put(u, ParseHTML(u, parsedURL.Host, resp.Body))
			_ = resp.Body.Close()
			cancel()
		case <-b.ctx.Done():
			return
		}
	}
}

func (b *Backend) GetFeed() ([]*FeedObject, error) {
	feed, err := b.fecli.Feed(context.TODO())
	if err != nil {
		return nil, err
	}
	nfeed := make([]*FeedObject, 0, len(feed))
	for _, fo := range feed {
		tfo := &FeedObject{
			Address:   fo.Address,
			ID:        fo.TxID.String(),
			Timestamp: fo.Timestamp,
			Fee:       fmt.Sprintf("%s %s", hutils.FormatBalance(fo.Fee, tconsts.Decimals), tconsts.Symbol),

			Message: fo.Content.Message,
			URL:     fo.Content.URL,
		}
		if len(fo.Content.URL) > 0 {
			if m, ok := b.htmlCache.Get(fo.Content.URL); ok {
				tfo.URLMeta = m
			} else {
				b.htmlCache.Put(fo.Content.URL, nil) // ensure we don't refetch
				b.urlQueue <- fo.Content.URL
			}
		}
		nfeed = append(nfeed, tfo)
	}
	return nfeed, nil
}

func (b *Backend) Message(message string, url string) error {
	// Get latest feed info
	recipient, fee, err := b.fecli.FeedInfo(context.TODO())
	if err != nil {
		return err
	}
	recipientAddr, err := codec.ParseAddressBech32(tconsts.HRP, recipient)
	if err != nil {
		return err
	}

	// Encode data
	fc := &manager.FeedContent{
		Message: message,
		URL:     url,
	}
	data, err := json.Marshal(fc)
	if err != nil {
		return err
	}

	// Ensure have sufficient balance
	bal, err := b.tcli.Balance(b.ctx, b.addrStr, ids.Empty)
	if err != nil {
		return err
	}

	// Generate transaction
	_, tx, maxFee, err := b.cli.GenerateTransaction(b.ctx, b.parser, &actions.Transfer{
		To:    recipientAddr,
		Asset: ids.Empty,
		Value: fee,
		Memo:  data,
	}, b.factory)
	if err != nil {
		return fmt.Errorf("%w: unable to generate transaction", err)
	}
	if maxFee+fee > bal {
		return fmt.Errorf("insufficient balance (have: %s %s, want: %s %s)", hutils.FormatBalance(bal, tconsts.Decimals), tconsts.Symbol, hutils.FormatBalance(maxFee+fee, tconsts.Decimals), tconsts.Symbol)
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
