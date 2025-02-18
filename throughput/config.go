// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package throughput

import (
	"github.com/ava-labs/hypersdk/chain"
)

type Config struct {
	uris             []string
	authFactory      chain.AuthFactory
	sZipf            float64
	vZipf            float64
	txsPerSecond     int
	minTxsPerSecond  int
	txsPerSecondStep int
	numClients       int
	numAccounts      int
}

// Config used for E2E testing and CLI
func NewFastConfig(
	uris []string,
	authFactory chain.AuthFactory,
) *Config {
	return &Config{
		uris:             uris,
		authFactory:      authFactory,
		sZipf:            1.01,
		vZipf:            2.7,
		txsPerSecond:     500,
		minTxsPerSecond:  100,
		txsPerSecondStep: 200,
		numClients:       10,
		numAccounts:      25,
	}
}

// Config used for load testing script
func NewLongRunningConfig(uris []string, authFactory chain.AuthFactory) (*Config, error) {
	return &Config{
		uris:             uris,
		authFactory:      authFactory,
		sZipf:            1.0001,
		vZipf:            2.7,
		txsPerSecond:     100000,
		minTxsPerSecond:  2000,
		txsPerSecondStep: 1000,
		numClients:       10,
		numAccounts:      10000,
	}, nil
}

func NewConfig(
	uris []string,
	key chain.AuthFactory,
	sZipf float64,
	vZipf float64,
	txsPerSecond int,
	minTxsPerSecond int,
	txsPerSecondStep int,
	numClients int,
	numAccounts int,
) *Config {
	return &Config{
		uris,
		key,
		sZipf,
		vZipf,
		txsPerSecond,
		minTxsPerSecond,
		txsPerSecondStep,
		numClients,
		numAccounts,
	}
}
