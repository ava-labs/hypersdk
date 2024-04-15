package vilmo

import "github.com/ava-labs/avalanchego/utils/units"

const (
	batchBufferSize       = 16 // just need to be big enough for any binary numbers
	minDiskValueSize      = 64
	uselessDividerRecycle = 3
	forceRecycle          = 128 * units.MiB // TODO: make this tuneable
)
