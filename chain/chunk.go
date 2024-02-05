package chain

import (
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/utils"
)

type Chunk struct {
	Slot int64          `json:"slot"` // rounded to nearest 100ms
	Txs  []*Transaction `json:"txs"`

	Producer  ids.NodeID     `json:"producer"`
	Signer    *bls.PublicKey `json:"signer"`
	Signature *bls.Signature `json:"signature"`

	size int
	id   ids.ID
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
	p.PackFixedBytes(bls.PublicKeyToBytes(c.Signer))

	return p.Bytes(), p.Err()
}

func (c *Chunk) ID() (ids.ID, error) {
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

func (c *Chunk) Size() int {
	return c.size
}

func (c *Chunk) Marshal() ([]byte, error) {
	size := consts.Int64Len + consts.IntLen + codec.CummSize(c.Txs) + consts.NodeIDLen + bls.PublicKeyLen + bls.SignatureLen
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
	p.PackFixedBytes(bls.PublicKeyToBytes(c.Signer))
	p.PackFixedBytes(bls.SignatureToBytes(c.Signature))

	bytes := p.Bytes()
	if err := p.Err(); err != nil {
		return nil, err
	}
	c.size = len(bytes)
	return bytes, nil
}

func UnmarshalChunk(raw []byte, parser Parser) (*Chunk, error) {
	var (
		actionRegistry, authRegistry = parser.Registry()
		p                            = codec.NewReader(raw, consts.NetworkSizeLimit)
		c                            Chunk
	)
	c.id = utils.ToID(raw)
	c.size = len(raw)

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
	}

	// Parse signer
	p.UnpackNodeID(true, &c.Producer)
	pk := make([]byte, bls.PublicKeyLen)
	p.UnpackFixedBytes(bls.PublicKeyLen, &pk)
	signer, err := bls.PublicKeyFromBytes(pk)
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

type ChunkSignature struct {
	Chunk ids.ID `json:"chunk"`
	Slot  int64  `json:"slot"`

	Producer  ids.NodeID     `json:"producer"`
	Signer    *bls.PublicKey `json:"signer"`
	Signature *bls.Signature `json:"signature"`
}

func (c *ChunkSignature) Marshal() ([]byte, error) {
	size := consts.IDLen + consts.Int64Len + consts.NodeIDLen + bls.PublicKeyLen + bls.SignatureLen
	p := codec.NewWriter(size, consts.NetworkSizeLimit)

	p.PackID(c.Chunk)
	p.PackInt64(c.Slot)
	p.PackNodeID(c.Producer)
	p.PackFixedBytes(bls.PublicKeyToBytes(c.Signer))
	p.PackFixedBytes(bls.SignatureToBytes(c.Signature))

	return p.Bytes(), p.Err()
}

func UnmarshalChunkSignature(raw []byte) (*ChunkSignature, error) {
	var (
		p = codec.NewReader(raw, consts.NetworkSizeLimit)
		c ChunkSignature
	)

	p.UnpackID(true, &c.Chunk)
	c.Slot = p.UnpackInt64(false)
	p.UnpackNodeID(true, &c.Producer)
	pk := make([]byte, bls.PublicKeyLen)
	p.UnpackFixedBytes(bls.PublicKeyLen, &pk)
	signer, err := bls.PublicKeyFromBytes(pk)
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

	Producer  ids.NodeID     `json:"producer"`
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
	return consts.IDLen + consts.Int64Len + consts.NodeIDLen + codec.BytesLen(signers) + bls.SignatureLen
}

func (c *ChunkCertificate) Marshal() ([]byte, error) {
	p := codec.NewWriter(c.Size(), consts.NetworkSizeLimit)

	p.PackID(c.Chunk)
	p.PackInt64(c.Slot)
	p.PackNodeID(c.Producer)
	p.PackBytes(c.Signers.Bytes())
	p.PackFixedBytes(bls.SignatureToBytes(c.Signature))

	return p.Bytes(), p.Err()
}

func (c *ChunkCertificate) MarshalPacker(p *codec.Packer) error {
	p.PackID(c.Chunk)
	p.PackInt64(c.Slot)
	p.PackNodeID(c.Producer)
	p.PackBytes(c.Signers.Bytes())
	p.PackFixedBytes(bls.SignatureToBytes(c.Signature))
	return p.Err()
}

func (c *ChunkCertificate) Digest() ([]byte, error) {
	signers := c.Signers.Bytes()
	size := consts.IDLen + consts.Int64Len + consts.NodeIDLen + codec.BytesLen(signers)
	p := codec.NewWriter(size, consts.NetworkSizeLimit)

	p.PackID(c.Chunk)
	p.PackInt64(c.Slot)
	p.PackNodeID(c.Producer)
	p.PackBytes(signers)

	return p.Bytes(), p.Err()
}

func UnmarshalChunkCertificate(raw []byte) (*ChunkCertificate, error) {
	var (
		p = codec.NewReader(raw, consts.NetworkSizeLimit)
		c ChunkCertificate
	)

	p.UnpackID(true, &c.Chunk)
	c.Slot = p.UnpackInt64(false)
	p.UnpackNodeID(true, &c.Producer)
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
	p.UnpackNodeID(true, &c.Producer)
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
	Chunk    ids.ID     `json:"chunk"`
	Producer ids.NodeID `json:"producer"`

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

func (c *FilteredChunk) Marshal() ([]byte, error) {
	size := consts.IDLen + consts.NodeIDLen + consts.IntLen + codec.CummSize(c.Txs) + consts.Uint64Len
	p := codec.NewWriter(size, consts.NetworkSizeLimit)

	// Marshal header
	p.PackID(c.Chunk)
	p.PackNodeID(c.Producer)

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
