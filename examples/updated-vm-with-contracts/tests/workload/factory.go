package workload

import (
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/codec"
)

type factory struct {
	factories []*auth.ED25519Factory
	addrs     []codec.Address
}