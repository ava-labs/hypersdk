// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package program

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/bytecodealliance/wasmtime-go/v14"

	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/program"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

const Name = "logger"

type Import struct {
	log        logging.Logger
	imports    runtime.SupportedImports
	meter      runtime.Meter
	registered bool
}

func New(log logging.Logger, db state.Mutable, cfg *runtime.Config) *Import {
	return &Import{
		log: log,
	}
}

func (i *Import) Name() string {
	return Name
}

func (i *Import) Register(link program.Link, meter runtime.Meter, imports runtime.SupportedImports) error {
	if i.registered {
		return fmt.Errorf("import module already registered: %q", Name)
	}

	i.registered = true
	i.imports = imports
	i.meter = meter

	return link.RegisterFn(program.NewFiveParamImport(Name, "log", i.callProgramFn))
}

func (i *Import) logFn(
	caller *wasmtime.Caller,
	callerID int64,
	level,
	msgPtr,
	msgLen int32,
) int32 {
	memory := program.NewMemory(caller)
	msgBytes, err := memory.Range(uint32(msgPtr), uint32(msgLen))
	if err != nil {
		i.log.Error("failed to read memory", zap.Error(err))
		return -1
	}

	switch zapcore.Level(level) {

	case zapcore.InfoLevel:
		i.log.Info(string(msgBytes))
	case zapcore.WarnLevel:
		i.log.Warn(string(msgBytes))
	case zapcore.DebugLevel:
		i.log.Debug(string(msgBytes))
	case zapcore.ErrorLevel:
		i.log.Error(string(msgBytes))
	default:
		i.log.Error("unknown log level",
			zap.Int32("level", level),
		)
		return -1
	}
	return 0
}

func randInt32() (int32, error) {
	var n int32
	err := binary.Read(rand.Reader, binary.BigEndian, &n)
	if err != nil {
		return 0, err
	}
	return n, nil
}
