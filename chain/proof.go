package chain

import (
	"errors"

	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
)

// TODO: make an interface? seems unneeded if using HyperSDK
type Proof struct {
	Proofs     []*merkledb.Proof
	PathProofs []*merkledb.PathProof

	size uint64
}

// TODO: get values out to apply to MerkleDB

func (p *Proof) MaxUnits(Rules) uint64 {
	return p.size
}

func (p *Proof) AsyncVerify() error {
	// TODO: ensure valid proof
	return nil
}

func (p *Proof) Marshal(pk *codec.Packer) error {
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

func unmarshalProof(p *codec.Packer) (*Proof, error) {
	start := p.Offset()
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
			return nil, err
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
			return nil, err
		}
		pathProofs = append(pathProofs, &pathProof)
	}
	return &Proof{proofs, pathProofs, uint64(p.Offset() - start)}, nil
}
