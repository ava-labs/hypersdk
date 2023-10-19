package address

import (
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/crypto/secp256r1"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
)

// Address returns a bech32-encoded string of the provided
// [address].
func Address(address codec.ShortBytes) (string, error) {
	if !VerifyAccountFormat(address) {
		return "", ErrMalformedAccount
	}
	return codec.Address(consts.HRP, address)
}

// ParseAddress converts a bech32-encoded address into
// bytes (includes [authTypeID] prefix.
func ParseAddress(s string) (codec.ShortBytes, error) {
	laddr, err := codec.ParseAnyAddress(consts.HRP, s)
	if err != nil {
		return nil, err
	}
	addr, ok := TrimShortBytes(laddr)
	if !ok {
		return nil, ErrMalformedAccount
	}
	return addr, nil
}

// VerifyAccountFormat ensures that any [codec.ShortBytes] being
// used as an address start with a valid [authTypeID].
func VerifyAccountFormat(sb codec.ShortBytes) bool {
	l := sb.Len()
	if l < 1 {
		return false
	}
	authType := sb[0]
	addressLen := len(sb[1:])
	switch authType {
	case consts.ED25519ID:
		return addressLen == ed25519.PublicKeyLen
	case consts.SECP256R1ID:
		return addressLen == secp256r1.PublicKeyLen
	default:
		return false
	}
}

// TrimShortBytes can be used to shrink [codec.ShortBytes] to the right
// size after bech32 decoding.
func TrimShortBytes(sb codec.ShortBytes) (codec.ShortBytes, bool) {
	l := sb.Len()
	if l < 1 {
		return nil, false
	}
	authType := sb[0]
	addressLen := len(sb[1:])
	switch authType {
	case consts.ED25519ID:
		if addressLen < ed25519.PublicKeyLen {
			return nil, false
		}
		return sb[:1+ed25519.PublicKeyLen], true
	case consts.SECP256R1ID:
		if addressLen < secp256r1.PublicKeyLen {
			return nil, false
		}
		return sb[:1+secp256r1.PublicKeyLen], true
	default:
		return nil, false
	}
}
