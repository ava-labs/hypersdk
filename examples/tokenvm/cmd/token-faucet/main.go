// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	frpc "github.com/ava-labs/hypersdk/examples/tokenvm/cmd/token-faucet/rpc"
	trpc "github.com/ava-labs/hypersdk/examples/tokenvm/rpc"
	tutils "github.com/ava-labs/hypersdk/examples/tokenvm/utils"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/server"
	"github.com/ava-labs/hypersdk/utils"
)

var (
	allowedOrigins  = []string{"*"}
	allowedHosts    = []string{"*"}
	shutdownTimeout = 30 * time.Second
	httpConfig      = server.HTTPConfig{
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
)

type Config struct {
	HTTPHost string `json:"host"`
	HTTPPort int    `json:"port"`

	PrivateKey ed25519.PrivateKey `json:"privateKey"`

	TokenRPC              string `json:"tokenRPC"`
	Amount                uint64 `json:"amount"`
	StartDifficulty       uint16 `json:"startDifficulty"`
	SolutionsPerSalt      int    `json:"solutionsPerSalt"`
	TargetDurationPerSalt int64  `json:"targetDurationPerSalt"` // seconds
}

func main() {
	logFactory := logging.NewFactory(logging.Config{
		DisplayLevel: logging.Info,
	})
	l, err := logFactory.Make("main")
	if err != nil {
		utils.Outf("{{red}}unable to initialize logger{{/}}: %v\n", err)
		os.Exit(1)
	}
	log := l

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
	listenAddress := net.JoinHostPort(config.HTTPHost, fmt.Sprintf("%d", config.HTTPPort))
	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		utils.Outf("{{red}}cannot create listener{{/}}: %v\n", err)
		os.Exit(1)
	}
	srv, err := server.New("", log, listener, httpConfig, allowedOrigins, allowedHosts, shutdownTimeout)
	if err != nil {
		utils.Outf("{{red}}cannot create server{{/}}: %v\n", err)
		os.Exit(1)
	}

	// Add faucet handler
	faucetServer := frpc.NewJSONRPCServer(nil)
	handler, err := server.NewHandler(faucetServer, "faucet")
	if err != nil {
		utils.Outf("{{red}}cannot create handler{{/}}: %v\n", err)
		os.Exit(1)
	}
	if err := srv.AddRoute(&common.HTTPHandler{
		LockOptions: common.NoLock,
		Handler:     handler,
	}, &sync.RWMutex{}, "faucet", ""); err != nil {
		utils.Outf("{{red}}cannot add faucet route{{/}}: %v\n", err)
		os.Exit(1)
	}
	if err := srv.Dispatch(); err != nil {
		utils.Outf("{{red}}server exited ungracefully{{/}}: %v\n", err)
		os.Exit(1)
	}
}
