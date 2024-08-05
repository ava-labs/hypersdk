// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package externalsubscriber

import (
	"context"
	"encoding/json"
	"log"

	"github.com/ava-labs/avalanchego/ids"
	"google.golang.org/grpc"

	"github.com/ava-labs/hypersdk/examples/morpheusvm/genesis"
	"github.com/ava-labs/hypersdk/extension/indexer"

	mpb "github.com/ava-labs/hypersdk/examples/morpheusvm/proto"
)

var logger = log.Default()

type MorpheusSubscriber struct {
	*indexer.ExternalSubscriber
	morpheusClient mpb.MorpheusSubscriberClient
}

func NewMorpheusSubscriber(server string, networkID uint32, chainID ids.ID, g *genesis.Genesis) (*MorpheusSubscriber, error) {
	// Connect to external subscriber
	extSub, err := indexer.NewExternalSubscriber(server)
	if err != nil {
		return nil, err
	}
	// Send info necessary to create parser
	conn, err := grpc.Dial(server, grpc.WithBlock(), grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	logger.Println("Connected to: ", server)
	morpheusSubClient := mpb.NewMorpheusSubscriberClient(conn)
	genesisBytes, _ := json.Marshal(g)

	_, err = morpheusSubClient.Initialize(
		context.TODO(),
		&mpb.InitMsg{
			NetworkID: networkID,
			ChainID:   chainID[:],
			Genesis:   genesisBytes,
		},
	)
	if err != nil {
		return nil, err
	}

	return &MorpheusSubscriber{
		ExternalSubscriber: extSub,
		morpheusClient:     morpheusSubClient,
	}, nil
}
