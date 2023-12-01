// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/tokenvm/cmd/token-faucet/config"
	"github.com/ava-labs/hypersdk/examples/tokenvm/cmd/token-faucet/manager"
	frpc "github.com/ava-labs/hypersdk/examples/tokenvm/cmd/token-faucet/rpc"
	"github.com/ava-labs/hypersdk/server"
	"github.com/ava-labs/hypersdk/utils"
	"go.uber.org/zap"
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

func fatal(l logging.Logger, msg string, fields ...zap.Field) {
	l.Fatal(msg, fields...)
	os.Exit(1)
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
		fatal(log, "no config file specified")
	}
	configPath := os.Args[1]
	rawConfig, err := os.ReadFile(configPath)
	if err != nil {
		fatal(log, "cannot open config file", zap.String("path", configPath), zap.Error(err))
	}
	var c config.Config
	if err := json.Unmarshal(rawConfig, &c); err != nil {
		fatal(log, "cannot read config file", zap.Error(err))
	}

	// Create private key
	if len(c.PrivateKeyBytes) == 0 {
		priv, err := ed25519.GeneratePrivateKey()
		if err != nil {
			fatal(log, "cannot generate private key", zap.Error(err))
		}
		c.PrivateKeyBytes = priv[:]
		b, err := json.MarshalIndent(&c, "", "  ")
		if err != nil {
			fatal(log, "cannot marshal new config", zap.Error(err))
		}
		fi, err := os.Lstat(configPath)
		if err != nil {
			fatal(log, "cannot get file stats for config", zap.Error(err))
		}
		if err := os.WriteFile(configPath, b, fi.Mode().Perm()); err != nil {
			fatal(log, "cannot write new config", zap.Error(err))
		}
		log.Info("created new faucet address", zap.String("address", c.AddressBech32()))
	} else {
		log.Info("loaded faucet address", zap.String("address", c.AddressBech32()))
	}

	// Create server
	listenAddress := net.JoinHostPort(c.HTTPHost, fmt.Sprintf("%d", c.HTTPPort))
	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		fatal(log, "cannot create listener", zap.Error(err))
	}
	srv, err := server.New("", log, listener, httpConfig, allowedOrigins, allowedHosts, shutdownTimeout)
	if err != nil {
		fatal(log, "cannot create server", zap.Error(err))
	}

	// Start manager
	manager, err := manager.New(log, &c)
	if err != nil {
		fatal(log, "cannot create manager", zap.Error(err))
	}
	go func() {
		if err := manager.Run(context.Background()); err != nil {
			log.Error("manager error", zap.Error(err))
		}
	}()

	// Add faucet handler
	faucetServer := frpc.NewJSONRPCServer(manager)
	handler, err := server.NewHandler(faucetServer, "faucet")
	if err != nil {
		fatal(log, "cannot create handler", zap.Error(err))
	}
	if err := srv.AddRoute(handler, "faucet", ""); err != nil {
		fatal(log, "cannot add facuet route", zap.Error(err))
	}

	// Start server
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		log.Info("triggering server shutdown", zap.Any("signal", sig))
		_ = srv.Shutdown()
	}()
	log.Info("server exited", zap.Error(srv.Dispatch()))
}
