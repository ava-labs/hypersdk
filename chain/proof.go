package chain

import (
	"context"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
)

// TODO: make an interface? seems unneeded if using HyperSDK
type Proof struct {
	Root ids.ID

	Proofs     []*merkledb.Proof
	PathProofs []*merkledb.PathProof

	size uint64
}

// TODO: get values out to apply to MerkleDB

func (p *Proof) MaxUnits(Rules) uint64 {
	return p.size
}

func (p *Proof) AsyncVerify(ctx context.Context) error {
	for _, proof := range p.Proofs {
		if err := proof.Verify(ctx, p.Root); err != nil {
			return err
		}
	}
	for _, proof := range p.PathProofs {
		if err := proof.Verify(ctx, p.Root); err != nil {
			return err
		}
	}
	return nil
}

func (p *Proof) Marshal(pk *codec.Packer) error {
	pk.PackID(p.Root)
	pk.PackInt(len(p.Proofs))
	for _, proof := range p.Proofs {
		b, err := merkledb.Codec.EncodeProof(merkledb.Version, proof)
		if err != nil {
			return err
		}
		pk.PackBytes(b)
	}
	pk.PackInt(len(p.PathProofs))
	for _, proof := range p.PathProofs {
		b, err := merkledb.Codec.EncodePathProof(merkledb.Version, proof)
		if err != nil {
			return err
		}
		pk.PackBytes(b)
	}
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

func UnmarshalProof(p *codec.Packer) (*Proof, error) {
	start := p.Offset()
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
	return &Proof{root, proofs, pathProofs, uint64(p.Offset() - start)}, nil
}
