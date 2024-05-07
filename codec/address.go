// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import (
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/cb58"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
)

const (
	AddressLen = 33
	LIDLen     = 33

	// These consts are pulled from BIP-173: https://github.com/bitcoin/bips/blob/master/bip-0173.mediawiki
	fromBits      = 8
	toBits        = 5
	separatorLen  = 1
	checksumlen   = 6
	maxBech32Size = 90
)

type (
	LID     [LIDLen]byte // Long ID
	Address = LID
)

// Empty is a useful all zero value
var Empty = LID{}

// CreateLID returns [LID] made from concatenating
// some [i] with an [id].
//
// This is commonly used for creating an ActionID. We
// keep the index|txID format to keep consistency with
// how address construction works. Index is the index
// in the [Action] array of a transaction. txID is the
// ID of that transaction.
func CreateLID(i uint8, id ids.ID) LID {
	a := make([]byte, LIDLen)
	a[0] = i
	copy(a[1:], id[:])
	return LID(a)
}

// CreateAddress returns [Address] made from concatenating
// [typeID] with [id].
func CreateAddress(typeID uint8, id ids.ID) Address {
	a := make([]byte, AddressLen)
	a[0] = typeID
	copy(a[1:], id[:])
	return Address(a)
}

// AddressBech32 returns a Bech32 address from [hrp] and [p].
// This function uses avalanchego's FormatBech32 function.
func AddressBech32(hrp string, p Address) (string, error) {
	expanedNum := AddressLen * fromBits
	expandedLen := expanedNum / toBits
	if expanedNum%toBits != 0 {
		expandedLen++
	}
	addrLen := len(hrp) + separatorLen + expandedLen + checksumlen
	if addrLen > maxBech32Size {
		return "", fmt.Errorf("%w: max=%d, requested=%d", ErrInvalidSize, maxBech32Size, addrLen)
	}
	return address.FormatBech32(hrp, p[:])
}

// MustAddressBech32 returns a Bech32 address from [hrp] and [p] or panics.
func MustAddressBech32(hrp string, p Address) string {
	addr, err := AddressBech32(hrp, p)
	if err != nil {
		panic(err)
	}
	return addr
}

// ParseAddressBech32 parses a Bech32 encoded address string and extracts
// its [AddressBytes]. If there is an error reading the address or
// the hrp value is not valid, ParseAddress returns an error.
func ParseAddressBech32(hrp, saddr string) (Address, error) {
	phrp, p, err := address.ParseBech32(saddr)
	if err != nil {
		return Empty, err
	}
	if phrp != hrp {
		return Empty, ErrIncorrectHRP
	}
	// The parsed value may be greater than [minLength] because the
	// underlying Bech32 implementation requires bytes to each encode 5 bits
	// instead of 8 (and we must pad the input to ensure we fill all bytes):
	// https://github.com/btcsuite/btcd/blob/902f797b0c4b3af3f7196d2f5d2343931d1b2bdf/btcutil/bech32/bech32.go#L325-L331
	if len(p) < AddressLen {
		return Empty, ErrInsufficientLength
	}
	return Address(p[:AddressLen]), nil
}

func (l LID) String() string {
	s, _ := cb58.Encode(l[:])
	return s
}

// FromString is the inverse of LID.String()
func FromString(lidStr string) (LID, error) {
	bytes, err := cb58.Decode(lidStr)
	if err != nil {
		return LID{}, err
	}
	return LID(bytes), nil
}
