// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import "encoding/hex"

// ToHex converts a PrivateKey to a hex string.
func ToHex(b []byte) string {
	return hex.EncodeToString(b)
}

// LoadHex Converts hex encoded string into bytes. Returns
// an error if key is invalid.
func LoadHex(s string, expectedSize int) ([]byte, error) {
	bytes, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}
	if expectedSize != -1 && len(bytes) != expectedSize {
		return nil, ErrInvalidSize
	}
	return bytes, nil
}
