package utils

import (
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/crypto/secp256r1"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
)

func VerifyAccountFormat(sb codec.ShortBytes) bool {
	l := sb.Len()
	if l < 1 {
		return false
	}
	authType := sb[0]
	acctLen := len(sb[1:])
	switch authType {
	case consts.ED25519ID:
		return acctLen == ed25519.PublicKeyLen
	case consts.SECP256R1ID:
		return acctLen == secp256r1.PublicKeyLen
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
	acctLen := len(sb[1:])
	switch authType {
	case consts.ED25519ID:
		if acctLen < ed25519.PublicKeyLen {
			return nil, false
		}
		return sb[:1+ed25519.PublicKeyLen], true
	case consts.SECP256R1ID:
		if acctLen < secp256r1.PublicKeyLen {
			return nil, false
		}
		return sb[:1+secp256r1.PublicKeyLen], true
	default:
		return nil, false
	}
}

func Address(acct codec.ShortBytes) (string, error) {
	if !VerifyAccountFormat(acct) {
		return "", ErrMalformedAccount
	}
	return codec.Address(consts.HRP, acct)
}

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
