package rpc

import "github.com/ava-labs/hypersdk/examples/tokenvm/genesis"

type Controller interface {
	Genesis() *genesis.Genesis
}
