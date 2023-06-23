package chain

import (
	"context"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/compression"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/utils"
)

// TODO: make an interface? seems unneeded if using HyperSDK
type Proof struct {
	// compression is deterministic on same version: https://github.com/facebook/zstd/issues/2079
	// TODO: not safe to use in prod without way more controls

	Root ids.ID

	Proofs     []*merkledb.Proof
	PathProofs []*merkledb.PathProof
}

// TODO: get values out to apply to MerkleDB

func (p *Proof) Units(Rules) uint64 {
	// TODO: measure size
	return 1000
}

func (p *Proof) AsyncVerify(ctx context.Context) error {
	// TODO: enforce sorted order
	for i, proof := range p.Proofs {
		if err := proof.Verify(ctx, p.Root); err != nil {
			return fmt.Errorf("%w: proof %d failed", err, i)
		}
	}
	for i, proof := range p.PathProofs {
		if err := proof.Verify(ctx, p.Root); err != nil {
			return fmt.Errorf("%w: path proof %d failed", err, i)
		}
	}
	return nil
}

func (p *Proof) Marshal(pk *codec.Packer) error {
	// TODO: enforce sorted order
	innerCodec := codec.NewWriter(consts.MaxInt)
	innerCodec.PackID(p.Root)
	innerCodec.PackInt(len(p.Proofs))
	for _, proof := range p.Proofs {
		b, err := merkledb.Codec.EncodeProof(merkledb.Version, proof)
		if err != nil {
			return err
		}
		innerCodec.PackBytes(b)
	}
	innerCodec.PackInt(len(p.PathProofs))
	for _, proof := range p.PathProofs {
		b, err := merkledb.Codec.EncodePathProof(merkledb.Version, proof)
		if err != nil {
			return err
		}
		innerCodec.PackBytes(b)
	}
	cmp, err := compression.NewZstdCompressor(int64(consts.NetworkSizeLimit))
	if err != nil {
		return err
	}
	mini, err := cmp.Compress(innerCodec.Bytes())
	if err != nil {
		return err
	}
	pk.PackBytes(mini)
	utils.Outf(
		"{{yellow}}compression savings:{{/}} %.2f%% %d->%d\n",
		float64(len(innerCodec.Bytes())-len(mini))/float64(len(innerCodec.Bytes()))*100,
		len(innerCodec.Bytes()),
		len(mini),
	)
	return nil
}

func (p *Proof) State() (map[merkledb.Path]merkledb.Maybe[[]byte], map[merkledb.Path]merkledb.Maybe[*merkledb.Node]) {
	values := make(map[merkledb.Path]merkledb.Maybe[[]byte])
	for _, proof := range p.Proofs {
		values[merkledb.NewPath(proof.Key)] = proof.Value
	}
	nodes := make(map[merkledb.Path]merkledb.Maybe[*merkledb.Node])
	for _, proof := range p.PathProofs {
		key := proof.KeyPath.Deserialize()
		nodes[key] = proof.ToNode()
	}
	return values, nodes
}

func UnmarshalProof(op *codec.Packer) (*Proof, error) {
	dcmp, err := compression.NewZstdCompressor(int64(consts.NetworkSizeLimit))
	if err != nil {
		return nil, err
	}
	var raw []byte
	op.UnpackBytes(consts.MaxInt, true, &raw)
	mini, err := dcmp.Decompress(raw)
	if err != nil {
		return nil, err
	}
	p := codec.NewReader(mini, consts.MaxInt)
	var root ids.ID
	p.UnpackID(true, &root)
	proofCount := p.UnpackInt(true)
	proofs := []*merkledb.Proof{}
	for i := 0; i < proofCount; i++ {
		var b []byte
		p.UnpackBytes(consts.NetworkSizeLimit, true, &b)
		var proof merkledb.Proof
		v, err := merkledb.Codec.DecodeProof(b, &proof)
		if v != merkledb.Version {
			return nil, errors.New("invalid version")
		}
		if err != nil {
			return nil, fmt.Errorf("%w: unable to decode proof (%x)", err, b)
		}
		proofs = append(proofs, &proof)
	}
	pathProofCount := p.UnpackInt(true)
	pathProofs := []*merkledb.PathProof{}
	for i := 0; i < pathProofCount; i++ {
		var b []byte
		p.UnpackBytes(consts.NetworkSizeLimit, true, &b)
		var pathProof merkledb.PathProof
		v, err := merkledb.Codec.DecodePathProof(b, &pathProof)
		if v != merkledb.Version {
			return nil, errors.New("invalid version")
		}
		if err != nil {
			return nil, fmt.Errorf("%w: unable to decode path proof (%x)", err, b)
		}
		pathProofs = append(pathProofs, &pathProof)
	}
	return &Proof{root, proofs, pathProofs}, nil
}
