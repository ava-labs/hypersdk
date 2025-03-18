// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fixture

import (
	"strconv"

	"github.com/ava-labs/avalanchego/config"
	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"
)

func NewTmpnetNetwork(owner string, nodes []*tmpnet.Node, subnets ...*tmpnet.Subnet) *tmpnet.Network {
	gb := strconv.Itoa(1_073_741_824)
	tenGB := strconv.Itoa(10_737_418_240)
	return &tmpnet.Network{
		Owner: owner,
		Nodes: nodes,
		DefaultFlags: tmpnet.FlagsMap{
			config.ProposerVMUseCurrentHeightKey:               "true",
			config.InboundThrottlerVdrAllocSizeKey:             tenGB,
			config.InboundThrottlerAtLargeAllocSizeKey:         tenGB,
			config.InboundThrottlerNodeMaxAtLargeBytesKey:      tenGB,
			config.InboundThrottlerMaxProcessingMsgsPerNodeKey: strconv.Itoa(100000),
			config.InboundThrottlerBandwidthRefillRateKey:      gb,
			config.InboundThrottlerBandwidthMaxBurstSizeKey:    gb,
			config.CPUVdrAllocKey:                              strconv.Itoa(100000),
			config.DiskVdrAllocKey:                             strconv.Itoa(10737418240000),
			config.OutboundThrottlerVdrAllocSizeKey:            tenGB,
			config.OutboundThrottlerAtLargeAllocSizeKey:        tenGB,
			config.OutboundThrottlerNodeMaxAtLargeBytesKey:     tenGB,
			config.ConsensusAppConcurrencyKey:                  strconv.Itoa(512),
			config.ProfileContinuousEnabledKey:                 "true",
			config.ProfileContinuousFreqKey:                    "1m",
			config.HTTPAllowedHostsKey:                         "*",
		},
		Subnets: subnets,
	}
}
