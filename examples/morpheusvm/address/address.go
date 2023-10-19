package address

import (
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/crypto/secp256r1"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
)

// Bech32 returns a bech32-encoded string of the provided
// [address].
func Bech32(address codec.ShortBytes) (string, error) {
	if !VerifyFormat(address) {
		return "", ErrMalformed
	}
	return codec.Address(consts.HRP, address)
}

// MustBech32 returns a bech32-encoded string or panics.
func MustBech32(address codec.ShortBytes) string {
	addr, err := Bech32(address)
	if err != nil {
		panic(err)
	}
	return addr
}

// ParseBech32 converts a bech32-encoded address into
// bytes (includes [authTypeID] prefix).
func ParseBech32(s string) (codec.ShortBytes, error) {
	laddr, err := codec.ParseAnyAddress(consts.HRP, s)
	if err != nil {
		return nil, err
	}
	addr, ok := TrimBech32Bytes(laddr)
	if !ok {
		return nil, ErrMalformed
	}
	return addr, nil
}

// VerifyFormat ensures that any [codec.ShortBytes] being
// used as an address start with a valid [authTypeID].
func VerifyFormat(sb codec.ShortBytes) bool {
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

// TrimBech32Bytes can be used to shrink [codec.ShortBytes] to the right
// size after bech32 decoding.
func TrimBech32Bytes(sb codec.ShortBytes) (codec.ShortBytes, bool) {
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
