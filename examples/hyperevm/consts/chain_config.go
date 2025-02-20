// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package consts

import (
	"math/big"

	"github.com/ava-labs/subnet-evm/params"
)

var (
	ChainConfig  *params.ChainConfig
	DefaultRules params.Rules
)

func init() {
	ChainConfig = params.SubnetEVMDefaultChainConfig
	ChainConfig.ChainID = big.NewInt(0).Add(params.DefaultChainID, big.NewInt(1))

	DefaultRules = ChainConfig.Rules(new(big.Int), 0)
}
