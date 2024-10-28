// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import (
	"encoding/hex"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
)

// TODO: add a checksum to the hex address format (ideally similar to EIP55).
const AddressLen = 33

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
	b, err := LoadHex(s, AddressLen)
	if err != nil {
		return a, err
	}
	copy(a[:], b)
	return a, nil
}

// String implements fmt.Stringer.
func (a Address) String() string {
	return ToHex(a[:])
}

// MarshalText returns the hex representation of a.
func (a Address) MarshalText() ([]byte, error) {
	result := make([]byte, len(a)*2+2)
	copy(result, `0x`)
	hex.Encode(result[2:], a[:])
	return result, nil
}

// UnmarshalText parses a hex-encoded address.
func (a *Address) UnmarshalText(input []byte) error {
	// Check if the input has the '0x' prefix and skip it
	if len(input) >= 2 && input[0] == '0' && input[1] == 'x' {
		input = input[2:]
	}

	// Decode the hex string
	decoded, err := hex.DecodeString(string(input))
	if err != nil {
		return err // Return the error if the hex string is invalid
	}

	copy(a[:], decoded)
	return nil
}
