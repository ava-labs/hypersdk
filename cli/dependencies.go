package cli

import "github.com/ava-labs/hypersdk/crypto"

type Controller interface {
	Symbol() string
	ParseAddress(string) (crypto.PublicKey, error)
}
