// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
)

type ChainState interface {
	GetNetworkID() uint32
	GetSubnetID() ids.ID
	GetChainID() ids.ID
	GetCanonicalValidatorSet(ctx context.Context) (validators warp.CanonicalValidatorSet, err error)
	IsNodeValidator(ctx context.Context, nodeID ids.NodeID, pChainHeight uint64) (bool, error)
	SampleNodeID() ids.NodeID
	GetQuorumNum() uint64
	GetQuorumDen() uint64
}
