package chain

import (
	"errors"

	"github.com/ava-labs/hypersdk/codec"
)

// TODO: make an interface? seems unneeded if using HyperSDK
type Proof struct {
}

// TODO: get values out to apply to MerkleDB

func (p *Proof) MaxUnits(Rules) uint64 {
	return 0
}

func (p *Proof) AsyncVerify() error {
	// TODO: ensure valid proof
	return nil
}

func (p *Proof) Marshal(pk *codec.Packer) {
	return
}

func unmarshalProof(p *codec.Packer) (*Proof, error) {
	return nil, errors.New("not implemented")
}
