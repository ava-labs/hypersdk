// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/ava-labs/hypersdk/crypto/ed25519"
	trpc "github.com/ava-labs/hypersdk/examples/tokenvm/rpc"
	tutils "github.com/ava-labs/hypersdk/examples/tokenvm/utils"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/utils"
)

type Config struct {
	TokenRPC string `json:"tokenRPC"`
	Port     int    `json:"port"`

	PrivateKey ed25519.PrivateKey `json:"privateKey"`

	Amount                uint64 `json:"amount"`
	StartDifficulty       uint16 `json:"startDifficulty"`
	SolutionsPerSalt      int    `json:"solutionsPerSalt"`
	TargetDurationPerSalt int64  `json:"targetDurationPerSalt"` // seconds
}

func main() {
	// Load config
	if len(os.Args) != 2 {
		utils.Outf("{{red}}no config file specified{{/}}\n")
		os.Exit(1)
	}
	configPath := os.Args[1]
	rawConfig, err := os.ReadFile(configPath)
	if err != nil {
		utils.Outf("{{red}}cannot open config file (%s){{/}}: %v\n", configPath, err)
		os.Exit(1)
	}
	var config Config
	if err := json.Unmarshal(rawConfig, &config); err != nil {
		utils.Outf("{{red}}cannot read config file{{/}}: %v\n", err)
		os.Exit(1)
	}

	// Create private key
	if config.PrivateKey == ed25519.EmptyPrivateKey {
		priv, err := ed25519.GeneratePrivateKey()
		if err != nil {
			utils.Outf("{{red}}cannot generate private key{{/}}: %v\n", err)
			os.Exit(1)
		}
		config.PrivateKey = priv
		b, err := json.Marshal(&config)
		if err != nil {
			utils.Outf("{{red}}cannot marshal new config{{/}}: %v\n", err)
			os.Exit(1)
		}
		fi, err := os.Lstat(configPath)
		if err != nil {
			utils.Outf("{{red}}cannot get file stats for config{{/}}: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(configPath, b, fi.Mode().Perm()); err != nil {
			utils.Outf("{{red}}cannot update config{{/}}: %v\n", err)
			os.Exit(1)
		}
		utils.Outf("{{yellow}}created new private key{{/}}: %s\n", tutils.Address(priv.PublicKey()))
	}

	// Connect to Token API
	ctx := context.Background()
	cli := rpc.NewJSONRPCClient(config.TokenRPC)
	networkID, _, chainID, err := cli.Network(ctx)
	if err != nil {
		utils.Outf("{{red}}cannot get network{{/}}: %v\n", err)
		os.Exit(1)
	}
	scli, err := rpc.NewWebSocketClient(
		config.TokenRPC,
		rpc.DefaultHandshakeTimeout,
		pubsub.MaxPendingMessages,
		pubsub.MaxReadMessageSize,
	)
	if err != nil {
		utils.Outf("{{red}}cannot open ws connection{{/}}: %v\n", err)
		os.Exit(1)
	}
	tcli := trpc.NewJSONRPCClient(config.TokenRPC, networkID, chainID)

	// Create server
}
