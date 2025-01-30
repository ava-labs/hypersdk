// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fixture

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"
)

func NewHyperVMSubnet(name string, vmID ids.ID, genesisBytes []byte, nodes ...*tmpnet.Node) *tmpnet.Subnet {
	return &tmpnet.Subnet{
		Name: name,
		Chains: []*tmpnet.Chain{
			{
				VMID:    vmID,
				Genesis: genesisBytes,
				Config: `{
					"statesync": {
						"minBlocks": 128
					}
				}`,
			},
		},
		Config: tmpnet.FlagsMap{
			"proposerMinBlockDelay":       0,
			"proposerNumHistoricalBlocks": 50_000,
		},
		ValidatorIDs: tmpnet.NodesToIDs(nodes...),
	}
}
