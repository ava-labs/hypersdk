package auth

import (
	"errors"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/crypto/bls"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/crypto/secp256r1"
)

var (
	// ErrInvalidKeyType is returned when an invalid key type is provided
	ErrInvalidKeyType = errors.New("invalid key type")
)

// GetFactory returns the [chain.AuthFactory] for a given private key.
//
// A [chain.AuthFactory] signs transactions and provides a unit estimate
// for using a given private key (needed to estimate fees for a transaction).
func GetFactory(pk *PrivateKey) (chain.AuthFactory, error) {
	switch pk.Address[0] {
	case ED25519ID:
		return NewED25519Factory(ed25519.PrivateKey(pk.Bytes)), nil
	case SECP256R1ID:
		return NewSECP256R1Factory(secp256r1.PrivateKey(pk.Bytes)), nil
	case BLSID:
		p, err := bls.PrivateKeyFromBytes(pk.Bytes)
		if err != nil {
			return nil, err
		}
		return NewBLSFactory(p), nil
	default:
		return nil, ErrInvalidKeyType
	}
}