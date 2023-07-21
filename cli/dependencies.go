package cli

import "github.com/ava-labs/hypersdk/crypto"

type Controller interface {
	ParseAddress(string) (crypto.PublicKey, error)
}
