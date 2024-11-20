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

func NewDefaultConfig(
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
