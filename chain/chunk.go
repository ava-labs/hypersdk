package chain

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/utils"
	"go.uber.org/zap"
)

const chunkPrealloc = 16_384

type Chunk struct {
	Slot int64          `json:"slot"` // rounded to nearest 100ms
	Txs  []*Transaction `json:"txs"`

	Producer    ids.NodeID    `json:"producer"`
	Beneficiary codec.Address `json:"beneficiary"` // used for fees

	Signer    *bls.PublicKey `json:"signer"`
	Signature *bls.Signature `json:"signature"`

	id         ids.ID
	units      *Dimensions
	bytes      []byte
	authCounts map[uint8]int
}

func BuildChunk(ctx context.Context, vm VM) (*Chunk, error) {
	start := time.Now()
	now := time.Now().UnixMilli() - consts.ClockSkewAllowance
	sm := vm.StateManager()
	r := vm.Rules(now)
	c := &Chunk{
		Slot: utils.UnixRDeci(now, r.GetValidityWindow()), // chunk validity window is 9 seconds @todo
		Txs:  make([]*Transaction, 0, chunkPrealloc),
	}
	epoch := utils.Epoch(now, r.GetEpochDuration())

	// Don't build chunk if no P-Chain height for epoch
	timestamp, heights, err := vm.Engine().GetEpochHeights(ctx, []uint64{epoch})
	if err != nil {
		return nil, err
	}
	executedEpoch := utils.Epoch(timestamp, r.GetEpochDuration()) // epoch duration set to 10 seconds.
	if executedEpoch+2 < epoch {                                  // only require + 2 because we don't care about epoch + 1 like in verification.
		return nil, fmt.Errorf("executed epoch (%d) is too far behind (%d) to verify chunk", executedEpoch, epoch)
	}
	if heights[0] == nil {
		return nil, fmt.Errorf("no P-Chain height for epoch %d", epoch)
	}

	// Check if validator
	//
	// If not a validator in this epoch height, don't build.
	amValidator, err := vm.IsValidator(ctx, *heights[0], vm.NodeID())
	if err != nil {
		return nil, err
	}
	if !amValidator {
		return nil, ErrNotAValidator
	}

	// Pack chunk for build duration
	//
	// TODO: sort mempool by priority and fit (only fetch items that can be included)
	var (
		maxChunkUnits = r.GetMaxChunkUnits()
		chunkUnits    = Dimensions{}
		full          bool
		mempool       = vm.Mempool()
		authCounts    = make(map[uint8]int)

		restorableTxs = make([]*Transaction, 0, chunkPrealloc)
	)
	mempool.StartStreaming(ctx)
	for time.Since(start) < vm.GetTargetChunkBuildDuration() {
		tx, ok := mempool.Stream(ctx)
		if !ok {
			break
		}
		// Ensure we haven't included this transaction in a chunk yet
		//
		// Should protect us from issuing repeat txs (if others get duplicates,
		// there will be duplicate inclusion but this is fixed with partitions)
		if vm.IsIssuedTx(ctx, tx) {
			vm.RecordChunkBuildTxDropped()
			continue
		}

		// TODO: count outstanding for an account and ensure less than epoch bond
		// if too many, just put back into mempool and try again later

		// TODO: ensure tx can still be processed (bond not frozen)

		// TODO: skip if transaction will pay < max fee over validity window (this fee period or a future one based on limit
		// of activity).

		// TODO: check if chunk units greater than limit

		// TODO: verify transactions
		if tx.Base.Timestamp > c.Slot {
			vm.RecordChunkBuildTxDropped()
			continue
		}

		// Check if tx can fit in chunk
		txUnits, err := tx.Units(sm, r)
		if err != nil {
			vm.RecordChunkBuildTxDropped()
			vm.Logger().Warn("failed to get units for transaction", zap.Error(err))
			continue
		}
		nextUnits, err := Add(chunkUnits, txUnits)
		if err != nil || !maxChunkUnits.Greater(nextUnits) {
			full = true
			// TODO: update mempool to only provide txs that can fit in chunk
			// Want to maximize how "full" chunk is, so need to be a little complex
			// if there are transactions with uneven usage of resources.
			restorableTxs = append(restorableTxs, tx)
			vm.RecordRemainingMempool(vm.Mempool().Len(ctx))
			break
		}
		chunkUnits = nextUnits

		// Add transaction to chunk
		vm.IssueTx(ctx, tx) // prevents duplicate from being re-added to mempool
		c.Txs = append(c.Txs, tx)
		authCounts[tx.Auth.GetTypeID()]++
	}

	// Always close stream
	defer func() {
		if c.id == ids.Empty { // only happens if there is an error
			mempool.FinishStreaming(ctx, append(c.Txs, restorableTxs...))
			return
		}
		mempool.FinishStreaming(ctx, restorableTxs)
	}()

	// Discard chunk if nothing produced
	if len(c.Txs) == 0 {
		return nil, ErrNoTxs
	}

	// Setup chunk
	c.Producer = vm.NodeID()
	c.Beneficiary = vm.Beneficiary()
	c.Signer = vm.Signer()
	c.units = &chunkUnits
	c.authCounts = authCounts

	// Sign chunk
	digest, err := c.Digest()
	if err != nil {
		return nil, err
	}
	wm, err := warp.NewUnsignedMessage(r.NetworkID(), r.ChainID(), digest)
	if err != nil {
		return nil, err
	}
	sig, err := vm.Sign(wm)
	if err != nil {
		return nil, err
	}
	c.Signature, err = bls.SignatureFromBytes(sig)
	if err != nil {
		return nil, err
	}
	bytes, err := c.Marshal()
	if err != nil {
		return nil, err
	}
	c.id = utils.ToID(bytes)

	vm.Logger().Info(
		"built chunk with signature",
		zap.Stringer("nodeID", vm.NodeID()),
		zap.Uint32("networkID", r.NetworkID()),
		zap.Stringer("chainID", r.ChainID()),
		zap.Int64("slot", c.Slot),
		zap.Uint64("epoch", epoch),
		zap.Bool("full", full),
		zap.Int("txs", len(c.Txs)),
		zap.Any("units", chunkUnits),
		zap.String("signer", hex.EncodeToString(bls.PublicKeyToCompressedBytes(c.Signer))),
		zap.String("signature", hex.EncodeToString(bls.SignatureToBytes(c.Signature))),
		zap.Duration("t", time.Since(start)),
	)
	return c, nil
}

func (c *Chunk) Digest() ([]byte, error) {
	size := consts.Int64Len + consts.IntLen + codec.CummSize(c.Txs) + consts.NodeIDLen + bls.PublicKeyLen
	p := codec.NewWriter(size, consts.NetworkSizeLimit)

	// Marshal transactions
	p.PackInt64(c.Slot)
	p.PackInt(len(c.Txs))
	for _, tx := range c.Txs {
		if err := tx.Marshal(p); err != nil {
			return nil, err
		}
	}

	// Marshal signer
	p.PackNodeID(c.Producer)
	p.PackAddress(c.Beneficiary)
	p.PackFixedBytes(bls.PublicKeyToCompressedBytes(c.Signer))

	return p.Bytes(), p.Err()
}

func (c *Chunk) ID() ids.ID {
	return c.id
}

func (c *Chunk) Size() int {
	return consts.Int64Len + consts.IntLen + codec.CummSize(c.Txs) + consts.NodeIDLen + codec.AddressLen + bls.PublicKeyLen + bls.SignatureLen
}

func (c *Chunk) Units(sm StateManager, r Rules) (Dimensions, error) {
	if c.units != nil {
		return *c.units, nil
	}
	units := Dimensions{}
	for _, tx := range c.Txs {
		txUnits, err := tx.Units(sm, r)
		if err != nil {
			return Dimensions{}, err
		}
		nextUnits, err := Add(units, txUnits)
		if err != nil {
			return Dimensions{}, err
		}
		units = nextUnits
	}
	c.units = &units
	return units, nil
}

func (c *Chunk) Marshal() ([]byte, error) {
	if c.bytes != nil {
		return c.bytes, nil
	}

	p := codec.NewWriter(c.Size(), consts.NetworkSizeLimit)

	// Marshal transactions
	p.PackInt64(c.Slot)
	p.PackInt(len(c.Txs))
	for _, tx := range c.Txs {
		if err := tx.Marshal(p); err != nil {
			return nil, err
		}
	}

	// Marshal signer
	p.PackNodeID(c.Producer)
	p.PackAddress(c.Beneficiary)
	p.PackFixedBytes(bls.PublicKeyToCompressedBytes(c.Signer))
	p.PackFixedBytes(bls.SignatureToBytes(c.Signature))
	bytes, err := p.Bytes(), p.Err()
	if err != nil {
		return nil, err
	}
	c.bytes = bytes
	return bytes, nil
}

func (c *Chunk) VerifySignature(networkID uint32, chainID ids.ID) bool {
	digest, err := c.Digest()
	if err != nil {
		return false
	}
	// TODO: don't use warp message for this (nice to have chainID protection)?
	msg, err := warp.NewUnsignedMessage(networkID, chainID, digest)
	if err != nil {
		return false
	}
	return bls.Verify(c.Signer, c.Signature, msg.Bytes())
}

func (c *Chunk) AuthCounts() map[uint8]int {
	return c.authCounts
}

func UnmarshalChunk(raw []byte, parser Parser) (*Chunk, error) {
	var (
		actionRegistry, authRegistry = parser.Registry()
		p                            = codec.NewReader(raw, consts.NetworkSizeLimit)
		c                            Chunk
		authCounts                   = make(map[uint8]int)
	)
	c.id = utils.ToID(raw)

	// Parse transactions
	c.Slot = p.UnpackInt64(false)
	txCount := p.UnpackInt(true) // can't produce empty blocks
	c.Txs = []*Transaction{}     // don't preallocate all to avoid DoS
	for i := 0; i < txCount; i++ {
		tx, err := UnmarshalTx(p, actionRegistry, authRegistry)
		if err != nil {
			return nil, err
		}
		c.Txs = append(c.Txs, tx)
		authCounts[tx.Auth.GetTypeID()]++
	}
	c.authCounts = authCounts

	// Parse signer
	p.UnpackNodeID(true, &c.Producer)
	p.UnpackAddress(&c.Beneficiary)
	pk := make([]byte, bls.PublicKeyLen)
	p.UnpackFixedBytes(bls.PublicKeyLen, &pk)
	signer, err := bls.PublicKeyFromCompressedBytes(pk)
	if err != nil {
		return nil, fmt.Errorf("%w: unable to decompress pk=%x packerErr=%w", err, pk, p.Err())
	}
	c.Signer = signer
	sig := make([]byte, bls.SignatureLen)
	p.UnpackFixedBytes(bls.SignatureLen, &sig)
	signature, err := bls.SignatureFromBytes(sig)
	if err != nil {
		return nil, err
	}
	c.Signature = signature

	// Ensure no leftover bytes
	if !p.Empty() {
		return nil, fmt.Errorf("%w: remaining=%d", ErrInvalidObject, len(raw)-p.Offset())
	}
	return &c, p.Err()
}

type ChunkSignature struct {
	Chunk ids.ID `json:"chunk"`
	Slot  int64  `json:"slot"` // used for builders that don't yet have the chunk being sequenced to verify not included before expiry

	Signer    *bls.PublicKey `json:"signer"`
	Signature *bls.Signature `json:"signature"`
}

func (c *ChunkSignature) Size() int {
	return consts.IDLen + consts.Int64Len + bls.PublicKeyLen + bls.SignatureLen
}

func (c *ChunkSignature) Marshal() ([]byte, error) {
	size := c.Size()
	p := codec.NewWriter(size, consts.NetworkSizeLimit)

	p.PackID(c.Chunk)
	p.PackInt64(c.Slot)

	p.PackFixedBytes(bls.PublicKeyToCompressedBytes(c.Signer))
	p.PackFixedBytes(bls.SignatureToBytes(c.Signature))

	return p.Bytes(), p.Err()
}

func (c *ChunkSignature) Digest() ([]byte, error) {
	size := consts.IDLen + consts.Int64Len
	p := codec.NewWriter(size, consts.NetworkSizeLimit)

	p.PackID(c.Chunk)
	p.PackInt64(c.Slot)

	return p.Bytes(), p.Err()
}

func (c *ChunkSignature) VerifySignature(networkID uint32, chainID ids.ID) bool {
	digest, err := c.Digest()
	if err != nil {
		return false
	}
	// TODO: don't use warp message for this (nice to have chainID protection)?
	msg, err := warp.NewUnsignedMessage(networkID, chainID, digest)
	if err != nil {
		return false
	}
	return bls.Verify(c.Signer, c.Signature, msg.Bytes())
}

func UnmarshalChunkSignature(raw []byte) (*ChunkSignature, error) {
	var (
		p = codec.NewReader(raw, consts.NetworkSizeLimit)
		c ChunkSignature
	)

	p.UnpackID(true, &c.Chunk)
	c.Slot = p.UnpackInt64(false)
	pk := make([]byte, bls.PublicKeyLen)
	p.UnpackFixedBytes(bls.PublicKeyLen, &pk)
	signer, err := bls.PublicKeyFromCompressedBytes(pk)
	if err != nil {
		return nil, err
	}
	c.Signer = signer
	sig := make([]byte, bls.SignatureLen)
	p.UnpackFixedBytes(bls.SignatureLen, &sig)
	signature, err := bls.SignatureFromBytes(sig)
	if err != nil {
		return nil, err
	}
	c.Signature = signature

	// Ensure no leftover bytes
	if !p.Empty() {
		return nil, fmt.Errorf("%w: remaining=%d", ErrInvalidObject, len(raw)-p.Offset())
	}
	return &c, p.Err()
}

// TODO: which height to use to verify this signature?
// If we use the block context, validator set might change a bit too frequently?
type ChunkCertificate struct {
	Chunk ids.ID `json:"chunk"`
	Slot  int64  `json:"slot"`

	Signers   set.Bits       `json:"signers"`
	Signature *bls.Signature `json:"signature"`
}

// implements "emap.Item"
func (c *ChunkCertificate) ID() ids.ID {
	return c.Chunk
}

// implements "emap.Item"
func (c *ChunkCertificate) Expiry() int64 {
	return c.Slot
}

func (c *ChunkCertificate) Size() int {
	signers := c.Signers.Bytes()
	return consts.IDLen + consts.Int64Len + codec.BytesLen(signers) + bls.SignatureLen
}

func (c *ChunkCertificate) Marshal() ([]byte, error) {
	p := codec.NewWriter(c.Size(), consts.NetworkSizeLimit)

	p.PackID(c.Chunk)
	p.PackInt64(c.Slot)
	p.PackBytes(c.Signers.Bytes())
	p.PackFixedBytes(bls.SignatureToBytes(c.Signature))

	return p.Bytes(), p.Err()
}

func (c *ChunkCertificate) MarshalPacker(p *codec.Packer) error {
	p.PackID(c.Chunk)
	p.PackInt64(c.Slot)
	p.PackBytes(c.Signers.Bytes())
	p.PackFixedBytes(bls.SignatureToBytes(c.Signature))
	return p.Err()
}

// TODO: unify with ChunkSignature
func (c *ChunkCertificate) Digest() ([]byte, error) {
	size := consts.IDLen + consts.Int64Len
	p := codec.NewWriter(size, consts.NetworkSizeLimit)

	p.PackID(c.Chunk)
	p.PackInt64(c.Slot)

	return p.Bytes(), p.Err()
}

func (c *ChunkCertificate) VerifySignature(networkID uint32, chainID ids.ID, aggrPubKey *bls.PublicKey) bool {
	digest, err := c.Digest()
	if err != nil {
		return false
	}
	// TODO: don't use warp message for this (nice to have chainID protection)?
	msg, err := warp.NewUnsignedMessage(networkID, chainID, digest)
	if err != nil {
		return false
	}
	return bls.Verify(aggrPubKey, c.Signature, msg.Bytes())
}

func UnmarshalChunkCertificate(raw []byte) (*ChunkCertificate, error) {
	var (
		p = codec.NewReader(raw, consts.NetworkSizeLimit)
		c ChunkCertificate
	)

	p.UnpackID(true, &c.Chunk)
	c.Slot = p.UnpackInt64(false)
	var signerBytes []byte
	p.UnpackBytes(32 /* TODO: make const */, true, &signerBytes)
	c.Signers = set.BitsFromBytes(signerBytes)
	if len(signerBytes) != len(c.Signers.Bytes()) {
		return nil, fmt.Errorf("%w: signers not minimal", ErrInvalidObject)
	}
	sig := make([]byte, bls.SignatureLen)
	p.UnpackFixedBytes(bls.SignatureLen, &sig)
	signature, err := bls.SignatureFromBytes(sig)
	if err != nil {
		return nil, err
	}
	c.Signature = signature

	// Ensure no leftover bytes
	if !p.Empty() {
		return nil, fmt.Errorf("%w: remaining=%d", ErrInvalidObject, len(raw)-p.Offset())
	}
	return &c, p.Err()
}

func UnmarshalChunkCertificatePacker(p *codec.Packer) (*ChunkCertificate, error) {
	var c ChunkCertificate

	p.UnpackID(true, &c.Chunk)
	c.Slot = p.UnpackInt64(false)
	var signerBytes []byte
	p.UnpackBytes(32 /* TODO: make const */, true, &signerBytes)
	c.Signers = set.BitsFromBytes(signerBytes)
	if len(signerBytes) != len(c.Signers.Bytes()) {
		return nil, fmt.Errorf("%w: signers not minimal", ErrInvalidObject)
	}
	sig := make([]byte, bls.SignatureLen)
	p.UnpackFixedBytes(bls.SignatureLen, &sig)
	signature, err := bls.SignatureFromBytes(sig)
	if err != nil {
		return nil, err
	}
	c.Signature = signature

	return &c, nil
}

// TODO: consider evaluating what other fields should be here (tx results bit array? so no need to sync for simple transfers)
type FilteredChunk struct {
	Chunk ids.ID `json:"chunk"`
	Slot  int64  `json:"slot"`

	Producer    ids.NodeID    `json:"producer"`
	Beneficiary codec.Address `json:"beneficiary"` // used for fees

	Txs         []*Transaction `json:"txs"`
	WarpResults set.Bits64     `json:"warpResults"`

	id ids.ID
}

func (c *FilteredChunk) ID() (ids.ID, error) {
	if c.id != ids.Empty {
		return c.id, nil
	}

	bytes, err := c.Marshal()
	if err != nil {
		return ids.ID{}, err
	}
	c.id = utils.ToID(bytes)
	return c.id, nil
}

func (c *FilteredChunk) Size() int {
	return consts.IDLen + consts.Int64Len + consts.NodeIDLen + codec.AddressLen + consts.IntLen + codec.CummSize(c.Txs) + consts.Uint64Len
}

func (c *FilteredChunk) Marshal() ([]byte, error) {
	p := codec.NewWriter(c.Size(), consts.NetworkSizeLimit)

	// Marshal header
	p.PackID(c.Chunk)
	p.PackInt64(c.Slot)
	p.PackNodeID(c.Producer)
	p.PackAddress(c.Beneficiary)

	// Marshal transactions
	p.PackInt(len(c.Txs))
	for _, tx := range c.Txs {
		if err := tx.Marshal(p); err != nil {
			return nil, err
		}
	}
	p.PackUint64(uint64(c.WarpResults))

	return p.Bytes(), p.Err()
}

func UnmarshalFilteredChunk(raw []byte, parser Parser) (*FilteredChunk, error) {
	var (
		actionRegistry, authRegistry = parser.Registry()
		p                            = codec.NewReader(raw, consts.NetworkSizeLimit)
		c                            FilteredChunk
	)
	c.id = utils.ToID(raw)

	// Parse header
	p.UnpackID(true, &c.Chunk)
	c.Slot = p.UnpackInt64(false)
	p.UnpackNodeID(true, &c.Producer)
	p.UnpackAddress(&c.Beneficiary)

	// Parse transactions
	txCount := p.UnpackInt(false) // can produce empty filtered chunks
	c.Txs = []*Transaction{}      // don't preallocate all to avoid DoS
	for i := 0; i < txCount; i++ {
		tx, err := UnmarshalTx(p, actionRegistry, authRegistry)
		if err != nil {
			return nil, err
		}
		c.Txs = append(c.Txs, tx)
	}
	c.WarpResults = set.Bits64(p.UnpackUint64(false))

	// Ensure no leftover bytes
	if !p.Empty() {
		return nil, fmt.Errorf("%w: remaining=%d extra=%x err=%w", ErrInvalidObject, len(raw)-p.Offset(), raw[p.Offset():], p.Err())
	}
	return &c, p.Err()
}
