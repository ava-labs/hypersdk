// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import "github.com/ava-labs/hypersdk/consts"

const ShortBytesMaxSize = int(consts.MaxUint8)

type ShortBytes []byte

func (s ShortBytes) Len() int {
	return len(s)
}

func (s ShortBytes) Valid() bool {
	return len(s) <= ShortBytesMaxSize
}

func PrefixShortBytes(prefix byte, sb []byte) ShortBytes {
	b := make([]byte, 1+len(sb))
	b[0] = prefix
	copy(b[1:], sb)
	return ShortBytes(b)
}

func TrimShortBytesPrefix(sb ShortBytes) ShortBytes {
	if sb.Len() > 0 {
		return sb[1:]
	}
	return nil
}
