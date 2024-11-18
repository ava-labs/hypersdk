// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package throughput

import (
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/utils"
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

func NewE2EConfig(
	uris []string,
	key *auth.PrivateKey,
) *Config {
	return &Config{
		uris:             uris,
		key:              key,
		sZipf:            1.0001,
		vZipf:            2.7,
		txsPerSecond:     100000,
		minTxsPerSecond:  10000,
		txsPerSecondStep: 1000,
		numClients:       10,
		numAccounts:      100000,
	}
}

func NewDefaultCliConfig(uris []string) (*Config, error) {
	keyHex := "323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7"
	bytes, err := codec.LoadHex(keyHex, ed25519.PrivateKeyLen)
	if err != nil {
		utils.Outf("{{red:%s}}\n", err)
		return nil, err
	}
	privateKey := ed25519.PrivateKey(bytes)
	key := &auth.PrivateKey{
		Address: auth.NewED25519Address(privateKey.PublicKey()),
		Bytes:   bytes,
	}

	return &Config{
		uris:             uris,
		key:              key,
		sZipf:            1.0001,
		vZipf:            2.7,
		txsPerSecond:     100000,
		minTxsPerSecond:  15000,
		txsPerSecondStep: 1000,
		numClients:       10,
		// numAccounts: 10000000,
		numAccounts: 100000,
	}, nil
	// return &Config{
	// 	uris: uris,
	// 	key: key,
	// 	sZipf: 1.0001,
	// 	vZipf: 2.7,
	// 	txsPerSecond: 100000,
	// 	minTxsPerSecond: 15000,
	// 	txsPerSecondStep: 1000,
	// 	numClients: 10,
	// numAccounts: 10000000,
	// }, nil
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
