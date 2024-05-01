// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/vm"

	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/controller"
	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/genesis"
)

const (
	simulatorFolder = ".simulator"
)

type simulator struct {
	log      logging.Logger
	logLevel string

	vm      *vm.VM
	db      *state.SimpleMutable
	genesis *genesis.Genesis
	cleanup func()
}

func NewRootCmd() *cobra.Command {
	s := &simulator{}
	cmd := &cobra.Command{
		Use:   "simulator",
		Short: "HyperSDK program VM simulator",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	cmd.PersistentFlags().Bool("cleanup", false, "remove simulator directory on exit")

	cobra.EnablePrefixMatching = true
	cmd.CompletionOptions.HiddenDefaultCmd = true
	cmd.DisableAutoGenTag = true
	cmd.SilenceErrors = true
	cmd.SetHelpCommand(&cobra.Command{Hidden: true})
	cmd.PersistentFlags().StringVar(&s.logLevel, "log-level", "info", "log level")

	// initialize simulator vm
	err := s.Init()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// add subcommands
	cmd.AddCommand(
		newRunCmd(s.log, s.db),
		newProgramCmd(s.log, s.db),
		newKeyCmd(s.log, s.db),
	)

	// ensure vm and databases are properly closed on simulator exit
	cobra.OnFinalize(func() {
		if s.vm != nil {
			err := s.vm.Shutdown(cmd.Context())
			if err != nil {
				s.log.Error("simulator vm closed with error",
					zap.Error(err),
				)
			}
		}

		cleanup, _ := cmd.Flags().GetBool("cleanup")
		if cleanup {
			s.cleanup()
		}
	})

	return cmd
}

func (s *simulator) Init() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	b := make([]byte, 4)
	_, err = rand.Read(b)
	if err != nil {
		return err
	}

	// TODO: allow for user defined ids.
	nodeID := ids.BuildTestNodeID(b)
	networkID := uint32(1)
	subnetID := ids.GenerateTestID()
	chainID := ids.GenerateTestID()

	basePath := path.Join(homeDir, simulatorFolder)
	dbPath := path.Join(basePath, fmt.Sprintf("db-%s", nodeID.String()))

	loggingConfig := logging.Config{}
	loggingConfig.LogLevel, err = logging.ToLevel(s.logLevel)
	if err != nil {
		return err
	}
	loggingConfig.Directory = path.Join(basePath, fmt.Sprintf("logs-%s", nodeID.String()))
	loggingConfig.LogFormat = logging.JSON
	loggingConfig.DisableWriterDisplaying = true

	s.cleanup = func() {
		if err := os.RemoveAll(dbPath); err != nil {
			fmt.Fprintf(os.Stderr, "failed to remove simulator directory: %s\n", err)
		}

		if err := os.RemoveAll(loggingConfig.Directory); err != nil {
			fmt.Fprintf(os.Stderr, "failed to remove simulator logs: %s\n", err)
		}
	}

	// setup simulator logger
	logFactory := newLogFactory(loggingConfig)
	s.log, err = logFactory.Make("simulator")
	if err != nil {
		logFactory.Close()
		return err
	}

	sk, err := bls.NewSecretKey()
	if err != nil {
		return err
	}

	genesisBytes, err := json.Marshal(genesis.Default())
	if err != nil {
		return err
	}

	snowCtx := &snow.Context{
		NetworkID:    networkID,
		SubnetID:     subnetID,
		ChainID:      chainID,
		NodeID:       nodeID,
		Log:          logging.NoLog{}, // TODO: use real logger
		ChainDataDir: dbPath,
		Metrics:      metrics.NewOptionalGatherer(),
		PublicKey:    bls.PublicFromSecretKey(sk),
	}

	toEngine := make(chan common.Message, 1)

	// initialize the simulator VM
	vm := controller.New()

	err = vm.Initialize(
		context.TODO(),
		snowCtx,
		nil,
		genesisBytes,
		nil,
		nil,
		toEngine,
		nil,
		nil,
	)
	if err != nil {
		return err
	}
	s.vm = vm
	// force the vm to be ready because it has no peers.
	s.vm.ForceReady()

	stateDB, err := s.vm.State()
	if err != nil {
		return err
	}
	s.db = state.NewSimpleMutable(stateDB)
	s.genesis = genesis.Default()

	s.log.Info("simulator initialized",
		zap.String("log-level", s.logLevel),
	)

	return nil
}
