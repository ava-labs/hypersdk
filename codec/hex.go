// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
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
	if len(s) >= 2 && s[:2] == "0x" {
		s = s[2:]
	}

	bytes, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}
	if expectedSize != -1 && len(bytes) != expectedSize {
		return nil, ErrInvalidSize
	}
	return bytes, nil
}

type Bytes []byte

func (b Bytes) String() string {
	return ToHex(b)
}

// MarshalText returns the hex representation of b.
func (b Bytes) MarshalText() ([]byte, error) {
	return []byte(b.String()), nil
}

// UnmarshalText sets b to the bytes represented by text.
func (b *Bytes) UnmarshalText(text []byte) error {
	bytes, err := LoadHex(string(text), -1)
	if err != nil {
		return err
	}
	*b = bytes
	return nil
}
