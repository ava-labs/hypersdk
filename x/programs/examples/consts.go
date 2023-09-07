package examples

import (
	"os"

	"github.com/ava-labs/avalanchego/utils/logging"
)

const DBPath = "test.db"

var (
	// example cost map
	CostMap = map[string]uint64{
		"ConstI32 0x0": 1,
		"ConstI64 0x0": 2,
	}
	DefaultMaxFee uint64 = 10000
	log           = logging.NewLogger(
		"",
		logging.NewWrappedCore(
			logging.Debug,
			os.Stderr,
			logging.Plain.ConsoleEncoder(),
		))
)
