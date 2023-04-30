// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

type WarpSignature struct {
	PublicKey []byte `json:"publicKey"`
	Signature []byte `json:"signature"`
}

func NewWarpSignature(pk []byte, sig []byte) *WarpSignature {
	return &WarpSignature{pk, sig}
}
