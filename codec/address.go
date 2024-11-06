// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/hashing"
)

const (
	AddressLen  = 33
	checksumLen = 4
)

var (
	ErrBadChecksum     = errors.New("invalid input checksum")
	ErrMissingChecksum = errors.New("input string is smaller than the checksum size")
)

// Address represents the 33 byte address of a HyperSDK account
type Address [AddressLen]byte

var EmptyAddress = Address{}

// CreateAddress returns [Address] made from concatenating
// [typeID] with [id].
func CreateAddress(typeID uint8, id ids.ID) Address {
	a := make([]byte, AddressLen)
	a[0] = typeID
	copy(a[1:], id[:])
	return Address(a)
}

func ToAddress(b []byte) (Address, error) {
	var a Address
	if len(b) != AddressLen {
		return a, fmt.Errorf("failed to convert bytes to address: length of bytes is %d, expected %d", len(b), AddressLen)
	}
	copy(a[:], b)
	return a, nil
}

// StringToAddress returns Address with bytes set to the hex decoding
// of s.
// StringToAddress uses copy, which copies the minimum of
// either AddressLen or the length of the hex decoded string.
func StringToAddress(s string) (Address, error) {
	var a Address
	if err := a.UnmarshalText([]byte(s)); err != nil {
		return EmptyAddress, err
	}
	return a, nil
}

// String implements fmt.Stringer.
func (a Address) String() string {
	return encodeWithChecksum(a[:])
}

// MarshalText returns the checksummed hex representation of a.
func (a Address) MarshalText() ([]byte, error) {
	return []byte(encodeWithChecksum(a[:])), nil
}

// UnmarshalText parses a checksummed hex-encoded address
func (a *Address) UnmarshalText(input []byte) error {
	decoded, err := fromChecksum(string(input))
	if err != nil {
		return err
	}

	copy(a[:], decoded)
	return nil
}

// encodeWithChecksum appends a 4 byte checksum and encodes the result in hex format
func encodeWithChecksum(bytes []byte) string {
	bytesLen := len(bytes)
	checked := make([]byte, bytesLen+checksumLen)
	copy(checked, bytes)
	copy(checked[AddressLen:], hashing.Checksum(bytes, checksumLen))
	return "0x" + hex.EncodeToString(checked)
}

// fromChecksum interprets the hex bytes, validates the 4 byte checksum, and returns the original bytes
func fromChecksum(s string) ([]byte, error) {
	if len(s) >= 2 && s[0] == '0' && s[1] == 'x' {
		s = s[2:]
	}
	decoded, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}
	if len(decoded) < checksumLen {
		return nil, ErrMissingChecksum
	}
	// Verify checksum
	originalBytes := decoded[:len(decoded)-checksumLen]
	checksum := decoded[len(decoded)-checksumLen:]
	if !bytes.Equal(checksum, hashing.Checksum(originalBytes, checksumLen)) {
		return nil, ErrBadChecksum
	}
	return originalBytes, nil
}
