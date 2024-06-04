// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/akamensky/argparse"
	"github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/mattn/go-shellwords"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/vm"
	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/controller"
	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/genesis"
)

const (
	simulatorFolder = ".simulator"
)

const (
	LogDisplayLogsKey = "enable-stdout-logs"
	LogLevelKey       = "log-level"
	CleanupKey        = "cleanup"
	InterpreterKey    = "interpreter"
)

type Cmd interface {
	Run(context.Context, logging.Logger, *state.SimpleMutable, []string) (*Response, error)
	Happened() bool
}

type Simulator struct {
	log logging.Logger

	logLevel               *string
	cleanup                *bool
	enableWriterDisplaying *bool
	lastStep               int
	programIDStrMap        map[int]ids.ID

	vm      *vm.VM
	db      *state.SimpleMutable
	genesis *genesis.Genesis

	cleanupFn func()

	reader *bufio.Reader
}

func (s *Simulator) ParseCommandArgs(ctx context.Context, args []string, interpreterMode bool) error {
	parser, subcommands := s.BaseParser()

	// if it's our first time parsing args, there is the possibility to enter in interpreter mode
	if !interpreterMode {
		stdinCmd := InterpreterCmd{}
		stdinCmd.New(parser)
		subcommands = append(subcommands, &stdinCmd)
	}

	err := parser.Parse(args)
	if err != nil {
		fmt.Println(parser.Usage(err))
		return err
	}

	if !interpreterMode {
		if err := s.Init(); err != nil {
			return err
		}
	}
	s.log.Debug("simulator args", zap.Any("args", args))

	for _, cmd := range subcommands {
		if cmd.Happened() {
			resp, err := cmd.Run(ctx, s.log, s.db, args)
			if err != nil {
				return err
			}

			s.log.Debug("a step has ben ran", zap.Any("resp", resp))

			if interpreterMode {
				// we need feedback, so print response to stdout
				if err := resp.Print(); err != nil {
					return err
				}
			}

			if _, ok := cmd.(*InterpreterCmd); !ok && !interpreterMode {
				return nil
			}

			s.log.Debug("reading next cmd from stdin")
			readString, err := s.reader.ReadString('\n')
			if err == io.EOF {
				// happens when the caller dropped, we should stop here
				return nil
			} else if err != nil {
				s.log.Error("error while reading from stdin", zap.Error(err))
				return err
			}

			rawArgs := []string{"simulator"}
			parsed, err := shellwords.Parse(readString)
			if err != nil {
				return err
			}
			rawArgs = append(rawArgs, parsed...)

			err = s.ParseCommandArgs(ctx, rawArgs, true)
			if err != nil {
				return err
			} else {
				return nil
			}
		}
	}

	return errors.New("unreachable")
}

func (s *Simulator) Execute(ctx context.Context) error {
	s.lastStep = 0
	s.programIDStrMap = make(map[int]ids.ID)

	defer s.manageCleanup(ctx)

	err := s.ParseCommandArgs(ctx, os.Args, false)
	if err != nil {
		s.log.Error("error when parsing command args", zap.Error(err))
		return err
	}

	return nil
}

func (s *Simulator) manageCleanup(ctx context.Context) {
	// ensure vm and databases are properly closed on simulator exit
	if s.vm != nil {
		err := s.vm.Shutdown(ctx)
		if err != nil {
			s.log.Error("simulator vm closed with error",
				zap.Error(err),
			)
		}
	}

	if *s.cleanup {
		s.cleanupFn()
	}
}

func (s *Simulator) BaseParser() (*argparse.Parser, []Cmd) {
	parser := argparse.NewParser("simulator", "HyperSDK program VM simulator")
	s.cleanup = parser.Flag("", CleanupKey, &argparse.Options{Help: "remove simulator directory on exit", Default: true})
	s.logLevel = parser.String("", LogLevelKey, &argparse.Options{Help: "log level", Default: "info"})
	s.enableWriterDisplaying = parser.Flag("", LogDisplayLogsKey, &argparse.Options{Help: "enable displaying logs in stdout", Default: false})
	stdin := os.Stdin
	s.reader = bufio.NewReader(stdin)

	runCmd := runCmd{}
	runCmd.New(parser, s.programIDStrMap, &s.lastStep, s.reader)
	programCmd := programCreateCmd{}
	programCmd.New(parser)
	keyCmd := keyCreateCmd{}
	keyCmd.New(parser)

	return parser, []Cmd{&runCmd, &programCmd, &keyCmd}
}

func (s *Simulator) Init() error {
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
	dbPath := path.Join(basePath, "db-"+nodeID.String())

	loggingConfig := logging.Config{}
	typedLogLevel, err := logging.ToLevel(*s.logLevel)
	if err != nil {
		return err
	}
	loggingConfig.LogLevel = typedLogLevel
	loggingConfig.Directory = path.Join(basePath, "logs-"+nodeID.String())
	loggingConfig.LogFormat = logging.JSON
	loggingConfig.DisableWriterDisplaying = !*s.enableWriterDisplaying

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

	s.cleanupFn = func() {
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

	s.log.Info("simulator initialized",
		zap.String("log-level", *s.logLevel),
	)

	return nil
}
