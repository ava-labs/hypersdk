package orderbook

import (
	"github.com/ava-labs/avalanchego/utils/logging"
)

type Controller interface {
	Logger() logging.Logger
}
