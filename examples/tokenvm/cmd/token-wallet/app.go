// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"context"
	"runtime/debug"

	"github.com/ava-labs/hypersdk/examples/tokenvm/cmd/token-wallet/backend"

	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx context.Context
	log logger.Logger
	b   *backend.Backend
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		log: logger.NewDefaultLogger(),
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.b = backend.New(func(err error) {
		a.log.Error(err.Error())
		runtime.Quit(ctx)
	})
	if err := a.b.Start(ctx); err != nil {
		a.log.Error(err.Error())
		runtime.Quit(ctx)
	}
}

// shutdown is called after the frontend is destroyed.
func (a *App) shutdown(ctx context.Context) {
	if err := a.b.Shutdown(ctx); err != nil {
		a.log.Error(err.Error())
	}
}

func (a *App) GetLatestBlocks() []*backend.BlockInfo {
	return a.b.GetLatestBlocks()
}

func (a *App) GetTransactionStats() []*backend.GenericInfo {
	return a.b.GetTransactionStats()
}

func (a *App) GetAccountStats() []*backend.GenericInfo {
	return a.b.GetAccountStats()
}

func (a *App) GetUnitPrices() []*backend.GenericInfo {
	return a.b.GetUnitPrices()
}

func (a *App) GetChainID() string {
	return a.b.GetChainID()
}

func (a *App) GetMyAssets() []*backend.AssetInfo {
	return a.b.GetMyAssets()
}

func (a *App) GetBalance() ([]*backend.BalanceInfo, error) {
	return a.b.GetBalance()
}

func (a *App) CreateAsset(symbol string, decimals string, metadata string) error {
	return a.b.CreateAsset(symbol, decimals, metadata)
}

func (a *App) MintAsset(asset string, address string, amount string) error {
	return a.b.MintAsset(asset, address, amount)
}

func (a *App) Transfer(asset string, address string, amount string, memo string) error {
	return a.b.Transfer(asset, address, amount, memo)
}

func (a *App) GetAddress() string {
	return a.b.GetAddress()
}

func (a *App) GetTransactions() *backend.Transactions {
	return a.b.GetTransactions()
}

func (a *App) StartFaucetSearch() (*backend.FaucetSearchInfo, error) {
	return a.b.StartFaucetSearch()
}

func (a *App) GetFaucetSolutions() *backend.FaucetSolutions {
	return a.b.GetFaucetSolutions()
}

func (a *App) GetAddressBook() []*backend.AddressInfo {
	return a.b.GetAddressBook()
}

func (a *App) AddAddressBook(name string, address string) error {
	return a.b.AddAddressBook(name, address)
}

func (a *App) GetAllAssets() []*backend.AssetInfo {
	return a.b.GetAllAssets()
}

func (a *App) AddAsset(asset string) error {
	return a.b.AddAsset(asset)
}

func (a *App) GetMyOrders() ([]*backend.Order, error) {
	return a.b.GetMyOrders()
}

func (a *App) GetOrders(pair string) ([]*backend.Order, error) {
	return a.b.GetOrders(pair)
}

func (a *App) CreateOrder(assetIn string, inTick string, assetOut string, outTick string, supply string) error {
	return a.b.CreateOrder(assetIn, inTick, assetOut, outTick, supply)
}

func (a *App) FillOrder(orderID string, orderOwner string, assetIn string, inTick string, assetOut string, amount string) error {
	return a.b.FillOrder(orderID, orderOwner, assetIn, inTick, assetOut, amount)
}

func (a *App) CloseOrder(orderID string, assetOut string) error {
	return a.b.CloseOrder(orderID, assetOut)
}

func (a *App) GetFeedInfo() (*backend.FeedInfo, error) {
	return a.b.GetFeedInfo()
}

func (a *App) GetFeed() ([]*backend.FeedObject, error) {
	return a.b.GetFeed()
}

func (a *App) Message(message string, url string) error {
	return a.b.Message(message, url)
}

func (a *App) OpenLink(url string) {
	runtime.BrowserOpenURL(a.ctx, url)
}

func (a *App) GetCommitHash() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				return setting.Value
			}
		}
	}
	return ""
}
