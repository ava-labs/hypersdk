// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package logging

import (
	"github.com/ava-labs/avalanchego/utils/logging"
	"go.uber.org/zap"
)

var _ logging.Logger = (*Noop)(nil)

type Noop struct{}

func (*Noop) Debug(string, ...zap.Field) {}

func (*Noop) Enabled(logging.Level) bool {
	return false
}

func (*Noop) Error(string, ...zap.Field) {}

func (*Noop) Fatal(string, ...zap.Field) {}

func (*Noop) Info(string, ...zap.Field) {}

func (*Noop) RecoverAndExit(func(), func()) {}

func (*Noop) RecoverAndPanic(func()) {}

func (*Noop) SetLevel(logging.Level) {}

func (*Noop) Stop() {}

func (*Noop) StopOnPanic() {}

func (*Noop) Trace(string, ...zap.Field) {}

func (*Noop) Verbo(string, ...zap.Field) {}

func (*Noop) Warn(string, ...zap.Field) {}

func (*Noop) Write([]byte) (int, error) {
	return 0, nil
}
