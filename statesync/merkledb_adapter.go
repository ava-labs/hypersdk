// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package statesync

import (
	"context"
	"errors"
	"fmt"
	stdlibsync "sync"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/ava-labs/avalanchego/x/sync"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const namespace = "merklesyncer"

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

func (m *MerkleSyncer[T]) Wait(ctx context.Context) error { return m.syncManager.Wait(ctx) }

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

type MerkleOffsetAdapter[T MerkleSyncerBlock] struct {
	merkleSyncer  *MerkleSyncer[T]
	offset        uint64
	currentOffset uint64
	errOnce       stdlibsync.Once
	errCh         chan error
}

func NewMerkleOffsetAdapter[T MerkleSyncerBlock](
	offset uint64,
	merkleSyncer *MerkleSyncer[T],
) (*MerkleOffsetAdapter[T], error) {
	return &MerkleOffsetAdapter[T]{
		offset:       offset,
		merkleSyncer: merkleSyncer,
		errCh:        make(chan error, 1),
	}, nil
}

func (m *MerkleOffsetAdapter[T]) Start(ctx context.Context, target T) error {
	return nil
}

func (m *MerkleOffsetAdapter[T]) Wait(ctx context.Context) error {
	return <-m.errCh
}

func (m *MerkleOffsetAdapter[T]) Close() error {
	close(m.errCh)
	if m.merkleSyncer == nil {
		return nil
	}
	m.merkleSyncer.Close()
	return nil
}

func (m *MerkleOffsetAdapter[T]) delayedStart(ctx context.Context, target T) error {
	m.merkleSyncer.log.Info("Starting merkle syncer", zap.Uint64("offset", m.offset), zap.Stringer("target", target))
	startErr := m.merkleSyncer.Start(ctx, target)
	if startErr != nil {
		m.errOnce.Do(func() {
			m.errCh <- startErr
			close(m.errCh)
		})
		return startErr
	}

	go func() {
		m.errCh <- m.merkleSyncer.Wait(ctx)
		close(m.errCh)
	}()
	return nil
}

func (m *MerkleOffsetAdapter[T]) UpdateSyncTarget(ctx context.Context, target T) error {
	switch {
	case m.currentOffset < m.offset:
		m.currentOffset++
		return nil
	case m.currentOffset == m.offset:
		m.currentOffset++
		return m.delayedStart(ctx, target)
	default:
		return m.merkleSyncer.UpdateSyncTarget(ctx, target)
	}
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
