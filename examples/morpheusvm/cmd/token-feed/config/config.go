// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package config

import (
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/tokenvm/consts"
)

type Config struct {
	HTTPHost string `json:"host"`
	HTTPPort int    `json:"port"`

	TokenRPC string `json:"tokenRPC"`

	Recipient     string `json:"recipient"`
	recipientAddr codec.Address

	FeedSize               int    `json:"feedSize"`
	MinFee                 uint64 `json:"minFee"`
	FeeDelta               uint64 `json:"feeDelta"`
	MessagesPerEpoch       int    `json:"messagesPerEpoch"`
	TargetDurationPerEpoch int64  `json:"targetDurationPerEpoch"` // seconds
}

func (c *Config) RecipientAddress() (codec.Address, error) {
	if c.recipientAddr != codec.EmptyAddress {
		return c.recipientAddr, nil
	}
	addr, err := codec.ParseAddressBech32(consts.HRP, c.Recipient)
	if err == nil {
		c.recipientAddr = addr
	}
	return addr, err
}
