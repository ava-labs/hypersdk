// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/challenge"
	frpc "github.com/ava-labs/hypersdk/examples/tokenvm/cmd/token-faucet/rpc"
	"github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	trpc "github.com/ava-labs/hypersdk/examples/tokenvm/rpc"
	tutils "github.com/ava-labs/hypersdk/examples/tokenvm/utils"
	"github.com/ava-labs/hypersdk/rpc"
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
		utils.Outf("{{yellow}}created new faucet address{{/}}: %s\n", tutils.Address(priv.PublicKey()))
	}

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
	manager, err := NewManager(log, &config)
	if err != nil {
		utils.Outf("{{red}}cannot create manager{{/}}: %v\n", err)
		os.Exit(1)
	}
	faucetServer := frpc.NewJSONRPCServer(manager)
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

type Manager struct {
	log    logging.Logger
	config *Config

	cli  *rpc.JSONRPCClient
	tcli *trpc.JSONRPCClient

	factory *auth.ED25519Factory

	l            sync.RWMutex
	lastRotation int64
	salt         []byte
	difficulty   uint16
	solutions    set.Set[ids.ID]
}

func NewManager(logger logging.Logger, config *Config) (*Manager, error) {
	ctx := context.TODO()
	cli := rpc.NewJSONRPCClient(config.TokenRPC)
	networkID, _, chainID, err := cli.Network(ctx)
	if err != nil {
		return nil, err
	}
	tcli := trpc.NewJSONRPCClient(config.TokenRPC, networkID, chainID)
	m := &Manager{log: logger, config: config, cli: cli, tcli: tcli, factory: auth.NewED25519Factory(config.PrivateKey)}
	m.lastRotation = time.Now().Unix()
	m.difficulty = m.config.StartDifficulty
	m.solutions = set.NewSet[ids.ID](m.config.SolutionsPerSalt)
	m.salt, err = challenge.New()
	if err != nil {
		return nil, err
	}
	addr := tutils.Address(m.config.PrivateKey.PublicKey())
	bal, err := tcli.Balance(ctx, addr, ids.Empty)
	if err != nil {
		return nil, err
	}
	m.log.Info("faucet initialized",
		zap.String("address", addr),
		zap.Uint16("difficulty", m.difficulty),
		zap.String("balance", utils.FormatBalance(bal, consts.Decimals)),
	)
	return m, nil
}

func (m *Manager) GetFaucetAddress(_ context.Context) (ed25519.PublicKey, error) {
	return m.config.PrivateKey.PublicKey(), nil
}

func (m *Manager) GetChallenge(_ context.Context) ([]byte, uint16, error) {
	m.l.RLock()
	defer m.l.RUnlock()

	return m.salt, m.difficulty, nil
}

func (m *Manager) sendFunds(ctx context.Context, destination ed25519.PublicKey, amount uint64) (ids.ID, uint64, error) {
	parser, err := m.tcli.Parser(ctx)
	if err != nil {
		return ids.Empty, 0, err
	}
	submit, tx, maxFee, err := m.cli.GenerateTransaction(ctx, parser, nil, &actions.Transfer{
		To:    destination,
		Asset: ids.Empty,
		Value: amount,
	}, m.factory)
	if err != nil {
		return ids.Empty, 0, err
	}
	addr := tutils.Address(m.config.PrivateKey.PublicKey())
	bal, err := m.tcli.Balance(ctx, addr, ids.Empty)
	if err != nil {
		return ids.Empty, 0, err
	}
	if bal < maxFee+amount {
		// This is a "best guess" heuristic for balance as there may be txs in-flight.
		m.log.Warn("faucet has insufficient funds", zap.String("balance", utils.FormatBalance(bal, consts.Decimals)))
		return ids.Empty, 0, errors.New("insufficient balance")
	}
	return tx.ID(), maxFee, submit(ctx)
}

// TODO: increase difficulty if solutions/minute greater than target
func (m *Manager) SolveChallenge(ctx context.Context, solver ed25519.PublicKey, salt []byte, solution []byte) (ids.ID, error) {
	m.l.Lock()
	defer m.l.Unlock()

	// Ensure solution is valid
	if !bytes.Equal(m.salt, salt) {
		return ids.Empty, errors.New("salt expired")
	}
	if !challenge.Verify(salt, solution, m.difficulty) {
		return ids.Empty, errors.New("invalid solution")
	}
	solutionID := utils.ToID(solution)
	if m.solutions.Contains(solutionID) {
		return ids.Empty, errors.New("duplicate solution")
	}

	// Issue transaction
	txID, maxFee, err := m.sendFunds(ctx, solver, m.config.Amount)
	if err != nil {
		return ids.Empty, err
	}
	m.log.Info("fauceted funds",
		zap.Stringer("txID", txID),
		zap.String("max fee", utils.FormatBalance(maxFee, consts.Decimals)),
		zap.String("destination", tutils.Address(solver)),
		zap.String("amount", utils.FormatBalance(m.config.Amount, consts.Decimals)),
	)
	m.solutions.Add(solutionID)

	// Roll salt if stale
	if m.solutions.Len() < m.config.SolutionsPerSalt {
		return txID, nil
	}
	now := time.Now().Unix()
	elapsed := now - m.lastRotation
	if elapsed < int64(m.config.TargetDurationPerSalt) {
		m.difficulty++
		m.log.Info("increasing faucet difficulty", zap.Uint16("difficulty", m.difficulty))
	} else if m.difficulty > m.config.StartDifficulty {
		m.difficulty--
		m.log.Info("decreasing faucet difficulty", zap.Uint16("difficulty", m.difficulty))
	}
	m.lastRotation = time.Now().Unix()
	m.salt, err = challenge.New()
	if err != nil {
		// Should never happen
		return ids.Empty, err
	}
	m.solutions.Clear()
	return txID, nil
}
