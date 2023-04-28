package controller

import "github.com/ava-labs/hypersdk/examples/tokenvm/genesis"

func (c *Controller) Genesis() *genesis.Genesis {
	return c.genesis
}
