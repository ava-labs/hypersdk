// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package externalsubscriber

import (
	"context"
	"encoding/json"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/ava-labs/hypersdk/examples/morpheusvm/genesis"
	"github.com/ava-labs/hypersdk/extension/indexer"

	pb "github.com/ava-labs/hypersdk/proto"
)

type MorpheusSubscriber struct {
	*indexer.ExternalSubscriber
	client pb.ExternalSubscriberClient
}

func NewMorpheusSubscriber(server string, networkID uint32, chainID ids.ID, g *genesis.Genesis, log logging.Logger) (*MorpheusSubscriber, error) {
	// Connect to external subscriber
	extSub, err := indexer.NewExternalSubscriber(server)
	if err != nil {
		return nil, err
	}
	// Send info necessary to create parser
	conn, err := grpc.Dial(server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	log.Debug("Connected to: ", zap.Any("Server", server))
	client := pb.NewExternalSubscriberClient(conn)
	genesisBytes, _ := json.Marshal(g)

	_, err = client.Initialize(
		context.TODO(),
		&pb.InitRequest{
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
		client:             client,
	}, nil
}
