// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package config

type Config struct {
	HTTPHost string `json:"host"`
	HTTPPort int    `json:"port"`

	TokenRPC string `json:"tokenRPC"`

	Recipient              string `json:"recipient"`
	MinFee                 uint64 `json:"minFee"`
	FeeDelta               uint64 `json:"feeDelta"`
	MessagesPerEpoch       int    `json:"messagesPerEpoch"`
	TargetDurationPerEpoch int64  `json:"targetDurationPerEpoch"` // seconds
}
