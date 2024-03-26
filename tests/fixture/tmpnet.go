// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tests

import (
	"github.com/ava-labs/avalanchego/config"
	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"
)

func NewTmpnetNetwork(subnets ...*tmpnet.Subnet) *tmpnet.Network {
	return &tmpnet.Network{
		// Supply only non-default configuration to ensure that default values will be used.
		DefaultFlags: tmpnet.FlagsMap{
			config.ProposerVMUseCurrentHeightKey:               true,
			config.InboundThrottlerVdrAllocSizeKey:             10737418240,
			config.InboundThrottlerAtLargeAllocSizeKey:         10737418240,
			config.InboundThrottlerMaxProcessingMsgsPerNodeKey: 100000,
			config.InboundThrottlerBandwidthRefillRateKey:      1073741824,
			config.InboundThrottlerBandwidthMaxBurstSizeKey:    1073741824,
			config.CPUVdrAllocKey:                              100000,
			config.DiskVdrAllocKey:                             10737418240000,
			config.OutboundThrottlerVdrAllocSizeKey:            10737418240,
			config.OutboundThrottlerAtLargeAllocSizeKey:        10737418240,
			config.ConsensusGossipOnAcceptValidatorSizeKey:     10,
			config.ConsensusGossipOnAcceptPeerSizeKey:          10,
			config.ConsensusAppConcurrencyKey:                  512,
			config.ProfileContinuousEnabledKey:                 true,
			config.ProfileContinuousFreqKey:                    "1m",
			config.HTTPAllowedHostsKey:                         "*",
		},
		Subnets: subnets,
	}
}
