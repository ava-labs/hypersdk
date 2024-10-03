package loadgen

import (
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/crypto/bls"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/crypto/secp256r1"
)

// GetFactory returns the [chain.AuthFactory] for a given private key.
//
// A [chain.AuthFactory] signs transactions and provides a unit estimate
// for using a given private key (needed to estimate fees for a transaction).
func GetFactory(pk *auth.PrivateKey) (chain.AuthFactory, error) {
	switch pk.Address[0] {
	case auth.ED25519ID:
		return auth.NewED25519Factory(ed25519.PrivateKey(pk.Bytes)), nil
	case auth.SECP256R1ID:
		return auth.NewSECP256R1Factory(secp256r1.PrivateKey(pk.Bytes)), nil
	case auth.BLSID:
		p, err := bls.PrivateKeyFromBytes(pk.Bytes)
		if err != nil {
			return nil, err
		}
		return auth.NewBLSFactory(p), nil
	default:
		return nil, ErrInvalidKeyType
	}
}