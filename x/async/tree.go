// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package async

import (
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/codec"
)

var (
	errPastEpoch         = errors.New("epoch is in the past")
	errFutureEpoch       = errors.New("epoch too far in the future")
	errInsufficientFunds = errors.New("insufficient funds")
)

type EpochState interface {
	// GetBalance returns an immutable balance for the provided epoch + address pair
	// Errors are treated as fatal and a not found balance should be converted to 0.
	//
	// Each implementation must support fetching immutable balances to verify containers
	// in epoch N and N + 1.
	// Conventionally, this can be done by locking balances for epoch N at the close of
	// epoch N - 2 (the first time we may be aske to verify a block in epoch N).
	// Note: disjoint epochs are forbidden. In other words, a block whose parent is in
	// epoch N must be in either epoch N or N + 1. There are no time based jumps or skipped
	// epochs.
	GetBalance(epoch uint64, address codec.Address) (uint64, error)
}

type ChainIndex[T Tx] interface {
	GetContainer(containerID ids.ID) (Container[T], error)
}

// TODO: handle replay protection or document as external responsibility
type Tx interface {
	GetID() ids.ID
	Sponsor() codec.Address
	BandwidthFee() uint64
}

type Container[T Tx] interface {
	GetID() ids.ID
	GetParent() ids.ID
	GetHeight() uint64
	Epoch() uint64
	Txs() []T
}

// Tree utilizes Backend to support building and verifying batches of transactions
// that spend some amount of funds.
type Tree[T Tx] struct {
	chainIndex ChainIndex[T]

	// epochState must be implemented with only read access to the last accepted state ie
	// the state as of successfully executing tip.
	// We must be able to verify blocks in epoch N and N + 1, which requires an agreed on
	// notion of locked balances at the start of those epochs.
	// Internal to these epochs, we verify that the accumulated fees from all transactions
	// within the epoch (trace back to last block from prior epoch) are less than the locked
	// balance.
	epochState EpochState

	// tip represents our last accepted block (defines the current epoch) and is the
	// root of the processing tree.
	tip Container[T]
	// tipAccumulatedFees is a map of the accumulated fees from each sponsor from the
	// start of the epoch to the current tip.
	tipAccumulatedFees map[codec.Address]uint64

	// pendingContainers is a map containing all processing blocks that have been
	// verified but remain undecided.
	// Includes the accumulated fees for each container, so we never need to re-calculate when
	// traversing the history.
	pendingContainers map[ids.ID]pendingContainer[T]
}

type pendingContainer[T Tx] struct {
	container     Container[T]
	containerFees map[codec.Address]uint64
}

func NewTree[T Tx](
	chainIndex ChainIndex[T],
	epochState EpochState,
	tip Container[T],
) (*Tree[T], error) {
	var (
		tipEpoch           = tip.Epoch()
		parentID           = tip.GetParent()
		tipAccumulatedFees = make(map[codec.Address]uint64)
		tipFees            = make(map[codec.Address]uint64)
	)
	for _, tx := range tip.Txs() {
		tipFees[tx.Sponsor()] += tx.BandwidthFee()
	}

	for {
		ancestor, err := chainIndex.GetContainer(parentID)
		if err != nil {
			return nil, err
		}
		// If the ancestor was not in our epoch, stop traversing
		if ancestor.Epoch() < tipEpoch {
			break
		}

		// Sum the accumulated fees from all transactions in the ancestor
		for _, container := range ancestor.Txs() {
			tipAccumulatedFees[container.Sponsor()] += container.BandwidthFee()
		}
	}

	return &Tree[T]{
		chainIndex:         chainIndex,
		epochState:         epochState,
		tip:                tip,
		tipAccumulatedFees: tipAccumulatedFees,
		pendingContainers: map[ids.ID]pendingContainer[T]{
			tip.GetID(): {
				container:     tip,
				containerFees: tipFees,
			},
		},
	}, nil
}

func (t *Tree[T]) Verify(container Container[T]) error {
	containerEpoch := container.Epoch()
	if containerEpoch < t.tip.Epoch() {
		return fmt.Errorf("%w: %d < %d", errPastEpoch, containerEpoch, t.tip.Epoch())
	}
	if containerEpoch > t.tip.Epoch()+1 {
		return fmt.Errorf("%w: %d > tipEpoch + 1 = %d", errFutureEpoch, containerEpoch, t.tip.Epoch()+1)
	}
	parentPendingContainer, ok := t.pendingContainers[container.GetParent()]
	if !ok {
		return fmt.Errorf("parent container %s not found", container.GetParent())
	}

	pendingContainer := pendingContainer[T]{
		container:     container,
		containerFees: make(map[codec.Address]uint64),
	}
	// Sum of the proposed fees to include from this container and traversing back
	// to the last block in the epoch.
	proposedAccumulatedFees := make(map[codec.Address]uint64)
	for _, tx := range container.Txs() {
		pendingContainer.containerFees[tx.Sponsor()] = tx.BandwidthFee()
		proposedAccumulatedFees[tx.Sponsor()] += tx.BandwidthFee()
	}

	// Traverse the ancestors to calculate the accumulated fees
	for parentPendingContainer.container.Epoch() < container.Epoch() {
		for addr := range pendingContainer.containerFees {
			ancestorFee, ok := parentPendingContainer.containerFees[addr]
			if !ok {
				continue
			}
			proposedAccumulatedFees[addr] += ancestorFee
		}
	}

	// Verify that every transaction in the proposed container has enough funds
	// to cover the accumulated fees.
	for addr, proposedAccumulatedFee := range proposedAccumulatedFees {
		lockedBalance, err := t.epochState.GetBalance(container.Epoch(), addr)
		if err != nil {
			return err
		}
		if proposedAccumulatedFee > lockedBalance {
			return fmt.Errorf("%w proposed accumulated fee %d > locked balance %d",
				errInsufficientFunds,
				proposedAccumulatedFee,
				lockedBalance,
			)
		}
	}

	return nil
}

func (t *Tree[T]) Accept(container Container[T]) {
	acceptedContainer := t.pendingContainers[container.GetID()]
	delete(t.pendingContainers, container.GetID())

	// If we are in the next epoch, clear the accumulated fees through the end
	// of the past epoch.
	if container.Epoch() > t.tip.Epoch() {
		clear(t.tipAccumulatedFees)
	}

	for addr, fee := range acceptedContainer.containerFees {
		t.tipAccumulatedFees[addr] += fee
	}
	t.tip = container
}

func (t *Tree[T]) Reject(container Container[T]) {
	delete(t.pendingContainers, container.GetID())
}

func (t *Tree[T]) GetBalance(parent Container[T], epoch uint64, sponsor codec.Address) (uint64, error) {
	epochBalance, err := t.epochState.GetBalance(epoch, sponsor)
	if err != nil {
		return 0, err
	}

	// If my parent is the tip
	if parent.GetID() == t.tip.GetID() {
		return epochBalance - t.tipAccumulatedFees[sponsor], nil
	}

	if parent.Epoch() == epoch {
		return t.tipAccumulatedFees[sponsor], nil
	}
	if parent.Epoch() == epoch-1 {
		return t.tipAccumulatedFees[sponsor] - t.pendingContainers[parent.GetID()].containerFees[sponsor], nil
	}
	return t.epochState.GetBalance(epoch, sponsor)
}
