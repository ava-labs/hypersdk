package e2e

import (
	"encoding/json"
	"math"

	"github.com/ava-labs/hypersdk/examples/morpheusvm/genesis"
	"github.com/ava-labs/hypersdk/fees"
)

const prefundedAddrStr = "morpheus1qrzvk4zlwj9zsacqgtufx7zvapd3quufqpxk5rsdd4633m4wz2fdjk97rwu"

func DefaultGenesisValues() ([]byte, error) {
	gen := genesis.Default()
	// Set WindowTargetUnits to MaxUint64 for all dimensions to iterate full mempool during block building.
	gen.WindowTargetUnits = fees.Dimensions{math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64}
	// Set all lmiits to MaxUint64 to avoid limiting block size for all dimensions except bandwidth. Must limit bandwidth to avoid building
	// a block that exceeds the maximum size allowed by AvalancheGo.
	gen.MaxBlockUnits = fees.Dimensions{1800000, math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64}
	gen.MinBlockGap = 100
	gen.CustomAllocation = []*genesis.CustomAllocation{
		{
			Address: prefundedAddrStr,
			Balance: 10_000_000_000_000,
		},
	}
	genesisBytes, err := json.Marshal(gen)
	if err != nil {
		return nil, err
	}

	return genesisBytes, nil
}
