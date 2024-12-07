package auth

import (
	"fmt"
	"testing"

	"github.com/ava-labs/hypersdk/codec"

	"github.com/ava-labs/hypersdk/crypto/secp256k1"
)

const privateKey = "9c22ff5f21f0b81b113e63f7db6da94fedef11b2119b4088b89664fb9a3cb658"
const evmAddr = "0xC08B5542D177ac6686946920409741463a15dDdB"

func TestSECP256K1(t *testing.T) {
	priv, err := codec.LoadHex(privateKey, secp256k1.PrivateKeyLen)
	if err != nil {
		t.Fatal(err)
	}
	factory := NewSECP256K1Factory(secp256k1.PrivateKey(priv))

	addr := factory.Address()
	fmt.Println("addr: ", addr)
	fmt.Println("evmAddr: ", evmAddr)
}
