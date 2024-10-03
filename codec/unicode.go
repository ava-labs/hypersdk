// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

type UnicodeBytes []byte

func (b UnicodeBytes) MarshalText() ([]byte, error) {
	return []byte(b), nil
}

func (b *UnicodeBytes) UnmarshalText(text []byte) error {
	*b = text
	return nil
}
