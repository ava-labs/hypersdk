// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package bls

import (
	"errors"

	blst "github.com/supranational/blst/bindings/go"

	"github.com/ava-labs/avalanchego/utils/crypto/bls"
)

const SignatureLen = blst.BLST_P2_COMPRESS_BYTES

var (
	ErrFailedSignatureDecompress  = errors.New("couldn't decompress signature")
	errInvalidSignature           = errors.New("invalid signature")
	errNoSignatures               = errors.New("no signatures")
	errFailedSignatureAggregation = errors.New("couldn't aggregate signatures")
)

type (
	Signature          = blst.P2Affine
	AggregateSignature = blst.P2Aggregate
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