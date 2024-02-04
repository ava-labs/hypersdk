package chain

import (
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/utils"
	"golang.org/x/net/context"
)

type Chunk struct {
	// TODO: move to end
	Producer  ids.ShortID    `json:"producer"`
	Signer    *bls.PublicKey `json:"signer"`
	Signature *bls.Signature `json:"signature"`

	Expiry int64          `json:"expiry"`
	Txs    []*Transaction `json:"txs"`

	size int
	id   ids.ID
}

func (c *Chunk) SyntacticVerify(ctx context.Context) error {
	// TODO: verify signatures + duplicate + timestamp
	// TODO: just verify chunk signature (for posting fraud proof)?
	// TODO: don't verify signature until we know a chunk will be executed, otherwise it is a DoS
	return errors.New("not implemented")
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
	size := consts.ShortIDLen + consts.Uint64Len + consts.IntLen + codec.CummSize(c.Txs) + bls.PublicKeyLen + bls.SignatureLen
	p := codec.NewWriter(size, consts.NetworkSizeLimit)

	// Marshal header
	p.PackShortID(c.Producer)
	p.PackFixedBytes(bls.PublicKeyToBytes(c.Signer))
	p.PackFixedBytes(bls.SignatureToBytes(c.Signature))

	// Marsha transactions
	p.PackInt64(c.Expiry)
	p.PackInt(len(c.Txs))
	for _, tx := range c.Txs {
		if err := tx.Marshal(p); err != nil {
			return nil, err
		}
	}
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

	// Parse header
	p.UnpackShortID(true, &c.Producer)
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

	// Parse transactions
	c.Expiry = p.UnpackInt64(false)
	txCount := p.UnpackInt(true) // can't produce empty blocks
	c.Txs = []*Transaction{}     // don't preallocate all to avoid DoS
	for i := 0; i < txCount; i++ {
		tx, err := UnmarshalTx(p, actionRegistry, authRegistry)
		if err != nil {
			return nil, err
		}
		c.Txs = append(c.Txs, tx)
	}

	// Ensure no leftover bytes
	if !p.Empty() {
		return nil, fmt.Errorf("%w: remaining=%d", ErrInvalidObject, len(raw)-p.Offset())
	}
	return &c, p.Err()
}

type ChunkSignature struct {
	Chunk    ids.ID      `json:"chunk"`
	Producer ids.ShortID `json:"producer"`
	Expiry   int64       `json:"expiry"`

	Signer    *bls.PublicKey `json:"signer"`
	Signature *bls.Signature `json:"signature"`
}

func (c *ChunkSignature) Marshal() ([]byte, error) {
	size := consts.IDLen + consts.ShortIDLen + consts.Uint64Len + bls.PublicKeyLen + bls.SignatureLen
	p := codec.NewWriter(size, consts.NetworkSizeLimit)

	p.PackID(c.Chunk)
	p.PackShortID(c.Producer)
	p.PackInt64(c.Expiry)
	p.PackFixedBytes(bls.PublicKeyToBytes(c.Signer))
	p.PackFixedBytes(bls.SignatureToBytes(c.Signature))

	bytes := p.Bytes()
	if err := p.Err(); err != nil {
		return nil, err
	}
	return bytes, nil
}

func UnmarshalChunkSignature(raw []byte) (*ChunkSignature, error) {
	var (
		p = codec.NewReader(raw, consts.NetworkSizeLimit)
		c ChunkSignature
	)

	p.UnpackID(true, &c.Chunk)
	p.UnpackShortID(true, &c.Producer)
	c.Expiry = p.UnpackInt64(false)
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

type ChunkCertificate struct {
	Chunk    ids.ID      `json:"chunk"`
	Producer ids.ShortID `json:"producer"`
	Expiry   int64       `json:"expiry"`

	Signers   set.Bits       `json:"signers"`
	Signature *bls.Signature `json:"signature"`
}

func (c *ChunkCertificate) Marshal() ([]byte, error) {
	signers := c.Signers.Bytes()
	size := consts.IDLen + consts.ShortIDLen + consts.Uint64Len + codec.BytesLen(signers) + bls.SignatureLen
	p := codec.NewWriter(size, consts.NetworkSizeLimit)

	p.PackID(c.Chunk)
	p.PackShortID(c.Producer)
	p.PackInt64(c.Expiry)
	p.PackBytes(signers)
	p.PackFixedBytes(bls.SignatureToBytes(c.Signature))

	bytes := p.Bytes()
	if err := p.Err(); err != nil {
		return nil, err
	}
	return bytes, nil
}

func UnmarshalChunkCertificate(raw []byte) (*ChunkCertificate, error) {
	var (
		p = codec.NewReader(raw, consts.NetworkSizeLimit)
		c ChunkCertificate
	)

	p.UnpackID(true, &c.Chunk)
	p.UnpackShortID(true, &c.Producer)
	c.Expiry = p.UnpackInt64(false)
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
