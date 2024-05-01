// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"fmt"
	"io"
	"os"
	"path"
	"sync"

	"github.com/ava-labs/avalanchego/utils/logging"

	"go.uber.org/zap"

	"gopkg.in/natefinch/lumberjack.v2"
)

// This package is lifted from AvalancheGo to circumvent a bug where stdout can not be muted.

type logWrapper struct {
	logger       logging.Logger
	displayLevel zap.AtomicLevel
	logLevel     zap.AtomicLevel
}

type logFactory struct {
	config logging.Config
	lock   sync.RWMutex

	// For each logger created by this factory:
	// Logger name --> the logger.
	loggers map[string]logWrapper
}

// NewFactory returns a new instance of a Factory producing loggers configured with
// the values set in the [config] parameter
func newLogFactory(config logging.Config) *logFactory {
	return &logFactory{
		config:  config,
		loggers: make(map[string]logWrapper),
	}
}

// Assumes [f.lock] is held
func (f *logFactory) makeLogger(config logging.Config) (logging.Logger, error) {
	if _, ok := f.loggers[config.LoggerName]; ok {
		return nil, fmt.Errorf("logger with name %q already exists", config.LoggerName)
	}
	consoleEnc := logging.Colors.ConsoleEncoder()
	fileEnc := config.LogFormat.FileEncoder()

	var consoleWriter io.WriteCloser
	if config.DisableWriterDisplaying {
		consoleWriter = newDiscardWriteCloser()
	} else {
		consoleWriter = os.Stderr
	}

	consoleCore := logging.NewWrappedCore(config.LogLevel, consoleWriter, consoleEnc)
	consoleCore.WriterDisabled = config.DisableWriterDisplaying

	rw := &lumberjack.Logger{
		Filename:   path.Join(config.Directory, config.LoggerName+".log"),
		MaxSize:    config.MaxSize,  // megabytes
		MaxAge:     config.MaxAge,   // days
		MaxBackups: config.MaxFiles, // files
		Compress:   config.Compress,
	}
	fileCore := logging.NewWrappedCore(config.LogLevel, rw, fileEnc)
	prefix := config.LogFormat.WrapPrefix(config.MsgPrefix)

	l := logging.NewLogger(prefix, consoleCore, fileCore)
	f.loggers[config.LoggerName] = logWrapper{
		logger:       l,
		displayLevel: consoleCore.AtomicLevel,
		logLevel:     fileCore.AtomicLevel,
	}
	return l, nil
}

func (f *logFactory) Make(name string) (logging.Logger, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	config := f.config
	config.LoggerName = name
	return f.makeLogger(config)
}

func (f *logFactory) Close() {
	f.lock.Lock()
	defer f.lock.Unlock()

	for _, lw := range f.loggers {
		lw.logger.Stop()
	}
	f.loggers = nil
}

type discardWriteCloser struct {
	io.Writer
}

func newDiscardWriteCloser() *discardWriteCloser {
	return &discardWriteCloser{io.Discard}
}

// Close implements the io.Closer interface.
func (n *discardWriteCloser) Close() error {
	// Do nothing and return nil.
	return nil
}
