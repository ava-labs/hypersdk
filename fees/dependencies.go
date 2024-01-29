// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fees

type Metadata interface {
	GetMinUnitPrice() Dimensions
	GetUnitPriceChangeDenominator() Dimensions
	GetWindowTargetUnits() Dimensions
	GetMaxBlockUnits() Dimensions
}
