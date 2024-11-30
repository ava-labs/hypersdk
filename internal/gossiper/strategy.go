// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package gossiper

import (
	"context"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
)

type TargetStrategy[T any] interface {
	Target(ctx context.Context, txs []T) ([]GossipContainer[T], error)
}

type TargetProposerConfig struct {
	GossipProposerDiff  int
	GossipProposerDepth int
}

func DefaultTargetProposerConfig() TargetProposerConfig {
	return TargetProposerConfig{
		GossipProposerDiff:  4,
		GossipProposerDepth: 1,
	}
}

type TargetProposers[T any] struct {
	Validators ValidatorSet
	Config     TargetProposerConfig
}

func (g *TargetProposers[T]) Target(ctx context.Context, txs []T) ([]GossipContainer[T], error) {
	// Select next set of proposers and send gossip to them
	proposers, err := g.Validators.Proposers(
		ctx,
		g.Config.GossipProposerDiff,
		g.Config.GossipProposerDepth,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: unable to fetch proposers", err)
	}
	if proposers.Len() == 0 {
		return nil, errors.New("no proposers to gossip to")
	}
	recipients := set.NewSet[ids.NodeID](len(proposers))
	for proposer := range proposers {
		// Don't gossip to self
		if proposer == g.Validators.NodeID() {
			continue
		}
		recipients.Add(proposer)
	}

	return []GossipContainer[T]{
		{
			NodeIDs: recipients,
			Txs:     txs,
		},
	}, nil
}

type TxAssigner[T any] interface {
	AssignTx(ctx context.Context, tx T) (ids.NodeID, bool)
}

type TargetAssigner[T Tx] struct {
	NodeID   ids.NodeID
	Assigner TxAssigner[T]
}

func (t *TargetAssigner[T]) Target(ctx context.Context, txs []T) ([]GossipContainer[T], error) {
	targetedGossip := make(map[ids.NodeID][]T)
	for _, tx := range txs {
		nodeID, ok := t.Assigner.AssignTx(ctx, tx)
		if !ok || nodeID == t.NodeID {
			continue
		}
		targetedGossip[nodeID] = append(targetedGossip[nodeID], tx)
	}

	gossipContainers := make([]GossipContainer[T], 0, len(targetedGossip))
	for targetedNodeID, txs := range targetedGossip {
		gossipContainers = append(gossipContainers, GossipContainer[T]{
			NodeIDs: set.Of(targetedNodeID),
			Txs:     txs,
		})
	}
	return gossipContainers, nil
}
