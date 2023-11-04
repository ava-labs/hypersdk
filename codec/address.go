package codec

import (
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
)

const (
	AddressLen = 33

	// These consts are pulled from BIP-173: https://github.com/bitcoin/bips/blob/master/bip-0173.mediawiki
	fromBits      = 8
	toBits        = 5
	separatorLen  = 1
	checksumlen   = 6
	maxBech32Size = 90
)

type AddressBytes [AddressLen]byte

var EmptyAddressBytes = [AddressLen]byte{}

// PrefixID returns [AddressBytes] made from concatenating
// [typeID] with [id].
func PrefixID(typeID uint8, id ids.ID) AddressBytes {
	a := make([]byte, AddressLen)
	a[0] = typeID
	copy(a[1:], id[:])
	return AddressBytes(a)
}

// Address returns a Bech32 address from hrp and p.
// This function uses avalanchego's FormatBech32 function.
func Address(hrp string, p AddressBytes) (string, error) {
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

// ParseAddress parses a Bech32 encoded address string and extracts
// its [AddressBytes]. If there is an error reading the address or
// the hrp value is not valid, ParseAddress returns an error.
func ParseAddress(hrp, saddr string) (AddressBytes, error) {
	phrp, p, err := address.ParseBech32(saddr)
	if err != nil {
		return EmptyAddressBytes, err
	}
	if phrp != hrp {
		return EmptyAddressBytes, ErrIncorrectHRP
	}
	// The parsed value may be greater than [minLength] because the
	// underlying Bech32 implementation requires bytes to each encode 5 bits
	// instead of 8 (and we must pad the input to ensure we fill all bytes):
	// https://github.com/btcsuite/btcd/blob/902f797b0c4b3af3f7196d2f5d2343931d1b2bdf/btcutil/bech32/bech32.go#L325-L331
	if len(p) < AddressLen {
		return EmptyAddressBytes, ErrInsufficientLength
	}
	return AddressBytes(p[:AddressLen]), nil
}
