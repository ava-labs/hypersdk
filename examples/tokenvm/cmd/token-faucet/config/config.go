// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package config

import "github.com/ava-labs/hypersdk/crypto/ed25519"

type Config struct {
	HTTPHost string `json:"host"`
	HTTPPort int    `json:"port"`

	PrivateKeyBytes []byte `json:"privateKeyBytes"`

	TokenRPC              string `json:"tokenRPC"`
	Amount                uint64 `json:"amount"`
	StartDifficulty       uint16 `json:"startDifficulty"`
	SolutionsPerSalt      int    `json:"solutionsPerSalt"`
	TargetDurationPerSalt int64  `json:"targetDurationPerSalt"` // seconds
}

func (c *Config) PrivateKey() ed25519.PrivateKey {
	return ed25519.PrivateKey(c.PrivateKeyBytes)
}
