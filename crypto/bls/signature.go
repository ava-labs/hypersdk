package bls

import "github.com/ava-labs/avalanchego/utils/crypto/bls"

const SignatureLen = bls.SignatureLen

type (
	Signature          = bls.Signature
	AggregateSignature = bls.AggregateSignature
)

func SignatureToBytes(sig *Signature) []byte {
	return bls.SignatureToBytes(sig)
}

func SignatureFromBytes(sigBytes []byte) (*Signature, error) {
	return bls.SignatureFromBytes(sigBytes)
}

func AggregateSignatures(sigs []*Signature) (*Signature, error) {
	return bls.AggregateSignatures(sigs)
}
