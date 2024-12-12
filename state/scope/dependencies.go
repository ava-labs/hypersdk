// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package scope

type RefundRules interface {
	GetStorageKeyReadRefundUnits() uint64
	GetStorageValueReadRefundUnits() uint64
}
