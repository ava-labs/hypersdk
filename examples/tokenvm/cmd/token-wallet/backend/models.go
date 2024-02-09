// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package backend

import (
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/fees"
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

type TransactionInfo struct {
	ID        string
	Size      string
	Timestamp int64
	Actor     string

	Success bool
	Type    string
	Units   string
	Fee     string
	Summary string
}

type TimeStat struct {
	Timestamp    int64
	Transactions int
	Accounts     set.Set[string]
	Prices       fees.Dimensions
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

type GenericInfo struct {
	Timestamp int64
	Count     uint64
	Category  string
}

type AssetInfo struct {
	ID string

	Symbol    string
	Decimals  int
	Metadata  string
	Supply    string
	Creator   string
	StrSymbol string
}

type BalanceInfo struct {
	ID string

	Str string
	Bal string
	Has bool
}

type Transactions struct {
	Alerts  []*Alert
	TxInfos []*TransactionInfo
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

type FaucetSolutions struct {
	Alerts        []*Alert
	CurrentSearch *FaucetSearchInfo
	PastSearches  []*FaucetSearchInfo
}

type Order struct {
	ID        string
	InID      string
	InSymbol  string
	OutID     string
	OutSymbol string

	// Set step value
	Price   string
	InTick  string
	OutTick string
	Rate    float64

	Remaining string
	Owner     string

	MaxInput  string
	InputStep string
}

type FeedInfo struct {
	Address string
	Fee     string
}

type FeedObject struct {
	Address   string
	ID        string
	Timestamp int64
	Fee       string

	Message string
	URL     string

	URLMeta *HTMLMeta
}
