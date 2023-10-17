package codec

import (
	"fmt"

	"github.com/ava-labs/avalanchego/utils/formatting/address"
)

const (
	fromBits      = 8
	toBits        = 5
	separatorLen  = 1
	checksumlen   = 6
	maxBech32Size = 90
)

// Address returns a Bech32 address from hrp and p.
// This function uses avalanchego's FormatBech32 function.
func Address(hrp string, p ShortBytes) (string, error) {
	if !p.Valid() {
		return "", ErrInvalidShortBytes
	}
	expanedNum := p.Len() * fromBits
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
// its bytes. If there is an error reading the address or the hrp
// value is not valid, ParseAddress returns an error.
func ParseAddress(hrp, saddr string, length int) (ShortBytes, error) {
	if length > ShortBytesMaxSize {
		return nil, ErrInvalidShortBytes
	}
	phrp, p, err := address.ParseBech32(saddr)
	if err != nil {
		return nil, err
	}
	if phrp != hrp {
		return nil, ErrIncorrectHRP
	}
	// The parsed value may be greater than [minLength] because the
	// underlying Bech32 implementation requires bytes to each encode 5 bits
	// instead of 8 (and we must pad the input to ensure we fill all bytes):
	// https://github.com/btcsuite/btcd/blob/902f797b0c4b3af3f7196d2f5d2343931d1b2bdf/btcutil/bech32/bech32.go#L325-L331
	if len(p) < length {
		return nil, ErrInsufficientLength
	}
	return p[:length], nil
}
