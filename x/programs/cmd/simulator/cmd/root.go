// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/x/merkledb"

	"github.com/ava-labs/hypersdk/pebble"
	"github.com/ava-labs/hypersdk/state"

	avatrace "github.com/ava-labs/avalanchego/trace"
)

const (
	simulatorFolder = ".simulator"
)

type simulator struct {
	log      logging.Logger
	logLevel string

	db      *state.SimpleMutable
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

	var stateDB database.Database
	var merkleDB merkledb.MerkleDB
	s.cleanup = func() {
		if err := os.RemoveAll(dbPath); err != nil {
			fmt.Fprintf(os.Stderr, "failed to remove simulator directory: %s\n", err)
		}

		if err := os.RemoveAll(loggingConfig.Directory); err != nil {
			fmt.Fprintf(os.Stderr, "failed to remove simulator logs: %s\n", err)
		}

		if merkleDB != nil {
			if err := merkleDB.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "failed to close simulator merkle db: %s\n", err)
			}
		}

		if stateDB != nil {
			if err := stateDB.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "failed to close simulator state db: %s\n", err)
			}
		}
	}

	// setup simulator logger
	logFactory := newLogFactory(loggingConfig)
	s.log, err = logFactory.Make("simulator")
	if err != nil {
		logFactory.Close()
		return err
	}

	stateDB, _, err = pebble.New(dbPath, pebble.NewDefaultConfig())
	if err != nil {
		return err
	}
	tracer, err := avatrace.New(avatrace.Config{Enabled: false})
	if err != nil {
		return err
	}

	merkleDB, err = merkledb.New(context.TODO(), stateDB, merkledb.Config{
		BranchFactor: merkledb.BranchFactor16,
		Tracer:       tracer,
	})
	if err != nil {
		return err
	}

	s.db = state.NewSimpleMutable(merkleDB)

	s.log.Info("simulator initialized",
		zap.String("log-level", s.logLevel),
	)

	return nil
}
