package rpc

import (
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/hypersdk/examples/tokenvm/genesis"
)

type Controller interface {
	Genesis() *genesis.Genesis
	Tracer() trace.Tracer
}
