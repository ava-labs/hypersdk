// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
)

func New(
	nodeID ids.NodeID,
	log logging.Logger,
	warpSigner warp.Signer,
	chainState ChainState,
	ruleFactory RuleFactory,
	db database.Database,
	chunkValidityWindow *validitywindow.TimeValidityWindow[EChunk],
	network *p2p.Network,
	getChunkProtocolID uint64,
	broadcastChunkCertProtocolID uint64,
	getChunkSignatureProtocolID uint64,
) (*Node, error) {
	// Construct the AvalancheGo AppClient based injected dep
	appClient := NewAppClient(
		log,
		ruleFactory,
		network,
		getChunkProtocolID,
		broadcastChunkCertProtocolID,
		getChunkSignatureProtocolID,
	)

	// Construct the node
	node, err := NewNode(
		nodeID,
		chainState,
		ruleFactory,
		chunkValidityWindow,
		db,
		appClient,
	)
	if err != nil {
		return nil, err
	}

	// Register the AvalancheGo p2p Handlers with the node as the injected
	// dependency
	if err := RegisterAppHandlers(
		node,
		warpSigner,
		network,
		getChunkProtocolID,
		broadcastChunkCertProtocolID,
		getChunkSignatureProtocolID,
	); err != nil {
		return nil, err
	}

	return node, nil
}
