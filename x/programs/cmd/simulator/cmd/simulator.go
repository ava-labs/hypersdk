package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"

	"github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/pebble"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/vm"
	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/controller"
	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/genesis"
	"go.uber.org/zap"
)

const (
	simulatorFolder = ".simulator"
)

func NewSimulator(logLevel string) *simulator {
	return &simulator{
		logLevel: logLevel,
	}
}

type simulator struct {
	log      logging.Logger
	logLevel string

	vm      *vm.VM
	db      *state.SimpleMutable
	genesis *genesis.Genesis
}

func (s *simulator) GracefulStop(ctx context.Context) {
	err := s.vm.Shutdown(ctx)
	if err != nil {
		s.log.Error("simulator vm closed with error",
			zap.Error(err),
		)
	}
}

func (s *simulator) Init() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	basePath := path.Join(homeDir, simulatorFolder)
	dbPath := path.Join(basePath, "db")

	// TODO: allow for user defined ids.
	nodeID := ids.GenerateTestNodeID()
	networkID := uint32(1)
	subnetID := ids.GenerateTestID()
	chainID := ids.GenerateTestID()

	loggingConfig := logging.Config{}
	loggingConfig.LogLevel, err = logging.ToLevel(s.logLevel)
	if err != nil {
		return fmt.Errorf("setting log level %v", err)
	}
	loggingConfig.Directory = path.Join(basePath, "logs")
	loggingConfig.LogFormat = logging.JSON
	loggingConfig.DisableWriterDisplaying = true

	// setup simulator logger
	logFactory := newLogFactory(loggingConfig)
	s.log, err = logFactory.Make("simulator")
	if err != nil {
		logFactory.Close()
		return nil
	}
	s.log.Info("simulator initializing",
		zap.String("log-level", s.logLevel),
	)

	sk, err := bls.NewSecretKey()
	if err != nil {
		return nil
	}

	// setup pebble and db manager
	pdb, _, err := pebble.New(dbPath, pebble.NewDefaultConfig())
	if err != nil {
		return nil
	}

	genesisBytes, err := json.Marshal(genesis.Default())
	if err != nil {
		return nil
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
		pdb,
		genesisBytes,
		nil,
		nil,
		toEngine,
		nil,
		nil,
	)
	if err != nil {
		return fmt.Errorf("vm init: %v", err)
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

func (s *simulator) CreateKey(ctx context.Context, name string) (ed25519.PublicKey, error) {
	return keyCreateFunc(ctx, s.db, name)
}
