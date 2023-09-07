// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package examples

import (
	"os"

	"github.com/ava-labs/avalanchego/utils/logging"
)

var (
	DBPath = "examples.db"
	// TODO: add more robust default
	// example cost map
	CostMap = map[string]uint64{
		"ConstI32 0x0": 1,
		"ConstI64 0x0": 2,
	}
	DefaultMaxFee uint64 = 10000
	// TODO: drop this and use NewLoggerWithLogLevel for now
	log = logging.NewLogger(
		"",
		logging.NewWrappedCore(
			logging.Debug,
			os.Stderr,
			logging.Plain.ConsoleEncoder(),
		))
)
