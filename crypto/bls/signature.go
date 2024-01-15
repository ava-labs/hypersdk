// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package bls

import (
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
)

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
