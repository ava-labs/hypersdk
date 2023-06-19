// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/utils"
)

type TxBlock struct {
	Prnt   ids.ID         `json:"parent"`
	Tmstmp int64          `json:"timestamp"`
	Hght   uint64         `json:"height"`
	Txs    []*Transaction `json:"txs"`

	// TEMP
	Issued int64 `json:"issued"`
}

// Stateless is defined separately from "Block"
// in case external packages needs use the stateful block
// without mocking VM or parent block
type StatelessTxBlock struct {
	*TxBlock `json:"block"`

	id    ids.ID
	t     time.Time
	bytes []byte

	txsSet set.Set[ids.ID]

	vm VM
}

func NewGenesisTxBlock() *TxBlock {
	return &TxBlock{}
}

func NewTxBlock(vm VM, parent *StatelessTxBlock, tmstmp int64) *StatelessTxBlock {
	return &StatelessTxBlock{
		TxBlock: &TxBlock{
			Tmstmp: tmstmp,
			Prnt:   parent.ID(),
			Hght:   parent.Hght + 1,
		},
		vm: vm,
	}
}

// populateTxs is only called on blocks we did not build
func (b *StatelessTxBlock) populateTxs(ctx context.Context) error {
	ctx, span := b.vm.Tracer().Start(ctx, "StatelessBlock.populateTxs")
	defer span.End()

	// Process transactions
	b.txsSet = set.NewSet[ids.ID](len(b.Txs))
	for _, tx := range b.Txs {
		if b.txsSet.Contains(tx.ID()) {
			return ErrDuplicateTx
		}
		b.txsSet.Add(tx.ID())
	}
	return nil
}

func ParseTxBlock(
	ctx context.Context,
	blk *TxBlock,
	source []byte,
	vm VM,
) (*StatelessTxBlock, error) {
	ctx, span := vm.Tracer().Start(ctx, "chain.ParseTxBlock")
	defer span.End()

	// Perform basic correctness checks before doing any expensive work
	if blk.Hght > 0 {
		if blk.Tmstmp >= time.Now().Add(FutureBound).Unix() {
			return nil, ErrTimestampTooLate
		}
		if len(blk.Txs) == 0 {
			return nil, ErrNoTxs
		}
	}

	if len(source) == 0 {
		actionRegistry, authRegistry := vm.Registry()
		nsource, err := blk.Marshal(actionRegistry, authRegistry)
		if err != nil {
			return nil, err
		}
		source = nsource
	}
	b := &StatelessTxBlock{
		TxBlock: blk,
		t:       time.Unix(blk.Tmstmp, 0),
		bytes:   source,
		vm:      vm,
		id:      utils.ToID(source),
	}

	// If we are parsing an older block, it will not be re-executed and should
	// not be tracked as a parsed block
	lastAccepted := b.vm.LastAcceptedBlock()
	if lastAccepted == nil || b.Hght <= lastAccepted.MaxTxHght() { // nil when parsing genesis
		return b, nil
	}

	// Populate hashes and tx set
	b.vm.RecordTxBlockIssuanceDiff(time.Since(time.UnixMilli(b.Issued)))
	return b, b.populateTxs(ctx)
}

// [initializeBuilt] is invoked after a block is built
func (b *StatelessTxBlock) initializeBuilt(
	ctx context.Context,
) error {
	_, span := b.vm.Tracer().Start(ctx, "StatelessTxBlock.initializeBuilt")
	defer span.End()

	blk, err := b.TxBlock.Marshal(b.vm.Registry())
	if err != nil {
		return err
	}
	b.bytes = blk
	b.id = utils.ToID(b.bytes)
	b.t = time.Unix(b.TxBlock.Tmstmp, 0)
	b.txsSet = set.NewSet[ids.ID](len(b.Txs))
	for _, tx := range b.Txs {
		b.txsSet.Add(tx.ID())
	}
	return nil
}

// implements "snowman.Block.choices.Decidable"
func (b *StatelessTxBlock) ID() ids.ID { return b.id }

func (b *StatelessTxBlock) Verify(ctx context.Context) error {
	ctx, span := b.vm.Tracer().Start(
		ctx, "StatelessTxBlock.Verify",
		oteltrace.WithAttributes(
			attribute.Int("txs", len(b.Txs)),
			attribute.Int64("height", int64(b.Hght)),
		),
	)
	defer span.End()

	var (
		log   = b.vm.Logger()
		r     = b.vm.Rules(b.Tmstmp)
		start = time.Now()
	)

	// Perform basic correctness checks before doing any expensive work
	switch {
	case b.Tmstmp >= time.Now().Add(FutureBound).Unix():
		return ErrTimestampTooLate
	case len(b.Txs) == 0:
		return ErrNoTxs
	}

	// Verify parent is verified and available
	var prntHght uint64
	if b.Hght > 0 {
		prntHght = b.Hght - 1
	}
	parent, err := b.vm.GetStatelessTxBlock(ctx, b.Prnt, prntHght)
	if err != nil {
		log.Warn("could not get parent", zap.Stringer("id", b.Prnt))
		return err
	}
	if b.Tmstmp < parent.Tmstmp {
		return ErrTimestampTooEarly
	}

	// Ensure tx cannot be replayed
	//
	// Before node is considered ready (emap is fully populated), this may return
	// false when other validators think it is true.
	oldestAllowed := b.Tmstmp - r.GetValidityWindow()
	if oldestAllowed < 0 {
		// Can occur if verifying genesis
		oldestAllowed = 0
	}
	dup, err := parent.IsRepeat(ctx, oldestAllowed, b.Txs)
	if err != nil {
		return err
	}
	if dup {
		return fmt.Errorf("%w: duplicate in ancestry", ErrDuplicateTx)
	}

	// We purposely avoid root calc (which would make this spikey)
	b.vm.RecordTxBlockVerify(time.Since(start))
	return nil
}

func (b *StatelessTxBlock) Accept(ctx context.Context) error {
	ctx, span := b.vm.Tracer().Start(ctx, "StatelessTxBlock.Accept")
	defer span.End()

	return nil
}

// implements "snowman.Block.choices.Decidable"
func (b *StatelessTxBlock) Reject(ctx context.Context) error {
	ctx, span := b.vm.Tracer().Start(ctx, "StatelessTxBlock.Reject")
	defer span.End()

	return nil
}

// implements "snowman.Block"
func (b *StatelessTxBlock) Parent() ids.ID { return b.TxBlock.Prnt }

// implements "snowman.Block"
func (b *StatelessTxBlock) Bytes() []byte { return b.bytes }

// implements "snowman.Block"
func (b *StatelessTxBlock) Height() uint64 { return b.TxBlock.Hght }

// implements "snowman.Block"
func (b *StatelessTxBlock) Timestamp() time.Time { return b.t }

func (b *StatelessTxBlock) IsRepeat(
	ctx context.Context,
	oldestAllowed int64,
	txs []*Transaction,
) (bool, error) {
	ctx, span := b.vm.Tracer().Start(ctx, "StatelessTxBlock.IsRepeat")
	defer span.End()

	// Early exit if we are already back at least [ValidityWindow]
	if b.Tmstmp < oldestAllowed {
		return false, nil
	}

	// If we are at an accepted block or genesis, we can use the emap on the VM
	// instead of checking each block
	if b.Hght <= b.vm.LastProcessedBlock().MaxTxHght() {
		return b.vm.IsRepeat(ctx, txs), nil
	}

	// Check if block contains any overlapping txs
	for _, tx := range txs {
		if b.txsSet.Contains(tx.ID()) {
			return true, nil
		}
	}
	var prntHght uint64
	if b.Hght > 0 {
		// TODO: consider making a helper
		prntHght = b.Hght - 1
	}
	prnt, err := b.vm.GetStatelessTxBlock(ctx, b.Prnt, prntHght)
	if err != nil {
		return false, err
	}
	return prnt.IsRepeat(ctx, oldestAllowed, txs)
}

func (b *StatelessTxBlock) GetTxs() []*Transaction {
	return b.Txs
}

func (b *StatelessTxBlock) GetTimestamp() int64 {
	return b.Tmstmp
}

func (b *TxBlock) Marshal(
	actionRegistry ActionRegistry,
	authRegistry AuthRegistry,
) ([]byte, error) {
	size := consts.IDLen + consts.Uint64Len + consts.Uint64Len + consts.IntLen
	for _, tx := range b.Txs {
		size += tx.Size()
	}
	size += consts.Uint64Len
	p := codec.NewWriter(size, consts.NetworkSizeLimit)

	p.PackID(b.Prnt)
	p.PackInt64(b.Tmstmp)
	p.PackUint64(b.Hght)

	p.PackInt(len(b.Txs))
	for _, tx := range b.Txs {
		if err := tx.Marshal(p, actionRegistry, authRegistry); err != nil {
			return nil, err
		}
	}

	p.PackInt64(b.Issued)
	return p.Bytes(), p.Err()
}

func UnmarshalTxBlock(raw []byte, parser Parser) (*TxBlock, error) {
	var (
		p = codec.NewReader(raw, consts.NetworkSizeLimit)
		b TxBlock
	)

	p.UnpackID(false, &b.Prnt)
	b.Tmstmp = p.UnpackInt64(false)
	b.Hght = p.UnpackUint64(false)

	// Parse transactions
	txCount := p.UnpackInt(false) // could be 0 in genesis
	actionRegistry, authRegistry := parser.Registry()
	b.Txs = []*Transaction{} // don't preallocate all to avoid DoS
	for i := 0; i < txCount; i++ {
		tx, err := UnmarshalTx(p, actionRegistry, authRegistry)
		if err != nil {
			return nil, err
		}
		b.Txs = append(b.Txs, tx)
	}

	b.Issued = p.UnpackInt64(false)
	if !p.Empty() {
		// Ensure no leftover bytes
		return nil, ErrInvalidObject
	}
	return &b, p.Err()
}
