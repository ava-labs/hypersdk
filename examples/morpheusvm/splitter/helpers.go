// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package splitter

import (
	"encoding/json"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/genesis"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
)

// Used as a lambda function for creating ExternalSubscriberServer parser
func ParserFactory(networkID uint32, chainID ids.ID, genesisBytes []byte) (chain.Parser, error) {
	var genesis genesis.Genesis
	if err := json.Unmarshal(genesisBytes, &genesis); err != nil {
		return nil, err
	}
	parser := rpc.NewParser(networkID, chainID, &genesis)
	return parser, nil
}
