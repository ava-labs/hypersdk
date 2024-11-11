// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package netextensions

import (
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/wrappers"

	avasync "github.com/ava-labs/avalanchego/x/sync"
)

const (
	RangeProofHandlerID  = 0x0
	ChangeProofHandlerID = 0x1
	TxGossipHandlerID    = 0x2
)

// RegisterNetworkHandlers registers RangeProof, ChangeProof, Gossiper Handler to the network
func RegisterNetworkHandlers(network *p2p.Network, logger logging.Logger, db avasync.DB, isReady isReadyFunc, handleAppGossip handleAppGossipFunc) error {
	var errs wrappers.Errs
	errs.Add(
		RegisterRangeProofHandler(network, logger, db),
		RegisterChangeProofHandler(network, logger, db),
		RegisterGossiperHandler(network, logger, isReady, handleAppGossip),
	)
	return errs.Err
}

func RegisterRangeProofHandler(network *p2p.Network, logger logging.Logger, db avasync.DB) error {
	return network.AddHandler(
		RangeProofHandlerID,
		avasync.NewGetRangeProofHandler(logger, db),
	)
}

func RegisterChangeProofHandler(network *p2p.Network, logger logging.Logger, db avasync.DB) error {
	return network.AddHandler(
		ChangeProofHandlerID,
		avasync.NewGetChangeProofHandler(logger, db),
	)
}

func RegisterGossiperHandler(network *p2p.Network, logger logging.Logger, isReady isReadyFunc, handleAppGossip handleAppGossipFunc) error {
	return network.AddHandler(
		TxGossipHandlerID,
		NewTxGossipHandler(logger, isReady, handleAppGossip),
	)
}
