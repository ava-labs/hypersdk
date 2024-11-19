// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package throughput

import (
	"errors"

	"github.com/ava-labs/hypersdk/auth"
)

type Config struct {
	uris             []string
	key              *auth.PrivateKey
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
	key *auth.PrivateKey,
) *Config {
	return &Config{
		uris:             uris,
		key:              key,
		sZipf:            1.01,
		vZipf:            2.7,
		txsPerSecond:     500,
		minTxsPerSecond:  100,
		txsPerSecondStep: 200,
		numClients:       10,
		numAccounts:      25,
	}
}

func NewThroughputConfig(uris []string, keyHex string) (*Config, error) {
	if len(uris) == 0 || len(keyHex) == 0 {
		return nil, errors.New("uris and keyHex must be non-empty")
	}

	key, err := auth.FromString(auth.ED25519ID, keyHex)
	if err != nil {
		return nil, err
	}

	return &Config{
		uris:             uris,
		key:              key,
		sZipf:            1.01,
		vZipf:            2.7,
		txsPerSecond:     25000,
		minTxsPerSecond:  2000,
		txsPerSecondStep: 2000,
		numClients:       10,
		numAccounts:      100000,
	}, nil
}

func NewConfig(
	uris []string,
	key *auth.PrivateKey,
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
