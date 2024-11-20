// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package throughput

import (
	"errors"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
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

// Default config used for E2E testing and CLI
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

// Default config for using the load test script.
func NewDefaultLoadTestConfig(uris []string, keyHex string) (*Config, error) {
	if len(uris) == 0 || len(keyHex) == 0 {
		return nil, errors.New("uris and keyHex must be non-empty")
	}

	bytes, err := codec.LoadHex(keyHex, ed25519.PrivateKeyLen)
	if err != nil {
		return nil, err
	}
	privateKey := ed25519.PrivateKey(bytes)
	key := &auth.PrivateKey{
		Address: auth.NewED25519Address(privateKey.PublicKey()),
		Bytes:   bytes,
	}
	
	if err != nil {
		return nil, err
	}

	return &Config{
		uris:             uris,
		key:              key,
		sZipf:            1.0001,
		vZipf:            2.7,
		txsPerSecond:     100000,
		minTxsPerSecond:  2000,
		txsPerSecondStep: 1000,
		numClients:       10,
		// numAccounts: 10000000,
		numAccounts: 100000,
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
