// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package statesync

import (
	"context"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/ava-labs/avalanchego/x/sync"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

var _ Syncer[MerkleSyncerBlock] = (*MerkleSyncer[MerkleSyncerBlock])(nil)

type MerkleSyncerBlock interface {
	fmt.Stringer
	GetStateRoot() ids.ID
}

type MerkleSyncer[T MerkleSyncerBlock] struct {
	log                                       logging.Logger
	registerer                                prometheus.Registerer
	merkleDB                                  merkledb.MerkleDB
	network                                   *p2p.Network
	rangeProofHandlerID, changeProofHandlerID uint64
	merkleBranchFactor                        merkledb.BranchFactor
	simultaneousWorkLimit                     int

	syncManager *sync.Manager
}

func NewMerkleSyncer[T MerkleSyncerBlock](
	log logging.Logger,
	merkleDB merkledb.MerkleDB,
	network *p2p.Network,
	rangeProofHandlerID uint64,
	changeProofHandlerID uint64,
	merkleBranchFactor merkledb.BranchFactor,
	simultaneousWorkLimit int,
	registry prometheus.Registerer,
) (*MerkleSyncer[T], error) {
	return &MerkleSyncer[T]{
		log:                   log,
		registerer:            registry,
		merkleDB:              merkleDB,
		network:               network,
		rangeProofHandlerID:   rangeProofHandlerID,
		changeProofHandlerID:  changeProofHandlerID,
		merkleBranchFactor:    merkleBranchFactor,
		simultaneousWorkLimit: simultaneousWorkLimit,
	}, nil
}

func (m *MerkleSyncer[T]) Start(ctx context.Context, target T) error {
	m.log.Info("Starting merkle syncer", zap.Stringer("target", target))
	syncManager, err := sync.NewManager(sync.ManagerConfig{
		BranchFactor:          m.merkleBranchFactor,
		DB:                    m.merkleDB,
		RangeProofClient:      m.network.NewClient(m.rangeProofHandlerID),
		ChangeProofClient:     m.network.NewClient(m.changeProofHandlerID),
		SimultaneousWorkLimit: m.simultaneousWorkLimit,
		Log:                   m.log,
		TargetRoot:            target.GetStateRoot(),
	}, m.registerer)
	if err != nil {
		return err
	}
	m.syncManager = syncManager
	return m.syncManager.Start(ctx)
}

func (m *MerkleSyncer[T]) Wait(ctx context.Context) error {
	return m.syncManager.Wait(ctx)
}

func (m *MerkleSyncer[T]) Close() error {
	m.syncManager.Close()
	return nil
}

func (m *MerkleSyncer[T]) UpdateSyncTarget(_ context.Context, target T) error {
	err := m.syncManager.UpdateSyncTarget(target.GetStateRoot())
	if err == sync.ErrAlreadyClosed {
		return nil
	}
	return err
}

func RegisterHandlers(
	log logging.Logger,
	network *p2p.Network,
	rangeProofHandlerID uint64,
	changeProofHandlerID uint64,
	db merkledb.MerkleDB,
) error {
	return errors.Join(
		network.AddHandler(
			rangeProofHandlerID,
			sync.NewGetRangeProofHandler(log, db),
		),
		network.AddHandler(
			changeProofHandlerID,
			sync.NewGetChangeProofHandler(log, db),
		),
	)
}
