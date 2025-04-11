// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chaintest

import (
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/genesis"
)

func RuleFactory() chain.RuleFactory {
	rules := genesis.NewDefaultRules()
	// we maximize the window target units and max block units to avoid fund exhaustion from fee spikes.
	rules.WindowTargetUnits = fees.Dimensions{20_000_000, consts.MaxUint64, consts.MaxUint64, consts.MaxUint64, consts.MaxUint64}
	rules.MaxBlockUnits = fees.Dimensions{20_000_000, consts.MaxUint64, consts.MaxUint64, consts.MaxUint64, consts.MaxUint64}
	rules.MinUnitPrice = fees.Dimensions{1, 1, 1, 1, 1}
	return &genesis.ImmutableRuleFactory{Rules: rules}
}
