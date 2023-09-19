// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package config

import (
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/tokenvm/utils"
)

type Config struct {
	HTTPHost string `json:"host"`
	HTTPPort int    `json:"port"`

	TokenRPC string `json:"tokenRPC"`

	Recipient          string `json:"recipient"`
	recipientPublicKey ed25519.PublicKey

	FeedSize               int    `json:"feedSize"`
	MinFee                 uint64 `json:"minFee"`
	FeeDelta               uint64 `json:"feeDelta"`
	MessagesPerEpoch       int    `json:"messagesPerEpoch"`
	TargetDurationPerEpoch int64  `json:"targetDurationPerEpoch"` // seconds
}

func (c *Config) RecipientPublicKey() (ed25519.PublicKey, error) {
	if c.recipientPublicKey != ed25519.EmptyPublicKey {
		return c.recipientPublicKey, nil
	}
	pk, err := utils.ParseAddress(c.Recipient)
	if err == nil {
		c.recipientPublicKey = pk
	}
	return pk, err
}
