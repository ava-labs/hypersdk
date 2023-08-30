package examples

import (
	"os"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
)

const DBPath = "test.db"

var (
	runtimePublicKey = ed25519.EmptyPublicKey
	// example cost map
	costMap = map[string]uint64{
		"ConstI32 0x0": 1,
		"ConstI64 0x0": 2,
	}
	maxGas uint64 = 13000
	log           = logging.NewLogger(
		"",
		logging.NewWrappedCore(
			logging.Debug,
			os.Stderr,
			logging.Plain.ConsoleEncoder(),
		))
)
