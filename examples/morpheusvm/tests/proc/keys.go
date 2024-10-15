package proc

import (
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
)

type Ed25519TestKey struct {
	PrivKey ed25519.PrivateKey
	Addr    codec.Address
	AuthFactory *auth.ED25519Factory
}

func newDefualtKeys() []*Ed25519TestKey {
	testKeys := make([]*Ed25519TestKey, len(ed25519HexKeys))
	for i, keyHex := range ed25519HexKeys {
		privBytes, err := codec.LoadHex(keyHex, ed25519.PrivateKeyLen)
		if err != nil {
			panic(err)
		}
		priv := ed25519.PrivateKey(privBytes)
		addr := auth.NewED25519Address(priv.PublicKey())
		
		testKey := &Ed25519TestKey{
			PrivKey: priv,
			Addr: addr,
			AuthFactory: auth.NewED25519Factory(priv),
		}
		testKeys[i] = testKey
	}

	return testKeys
}

func (e *Ed25519TestKey) GetPrivateKey() *auth.PrivateKey {
	return &auth.PrivateKey{
		Address: e.Addr,
		Bytes: e.PrivKey[:],
	}
}
// seperate into VM config struct later
// type VMConfig struct {
// 	Genesis *genesis.DefaultGenesis
// }
