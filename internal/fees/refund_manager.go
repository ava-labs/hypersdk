// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fees

import (
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/internal/math"
)

var _ RefundManager = (*HotKeysRefundManager)(nil)

type RefundRules interface {
	GetStorageKeyReadRefundUnits() uint64
	GetStorageValueReadRefundUnits() uint64
}

type RefundManager interface {
	Compute(r RefundRules) (fees.Dimensions, error)
}

type HotKeysRefundManager struct {
	hotKeys map[string]uint16
}

func NewHotKeysRefundManager(hotKeys map[string]uint16) *HotKeysRefundManager {
	return &HotKeysRefundManager{hotKeys: hotKeys}
}

func (m *HotKeysRefundManager) Compute(r RefundRules) (fees.Dimensions, error) {
	readsRefundOp := math.NewUint64Operator(0)
	for _, v := range m.hotKeys {
		readsRefundOp.Add(r.GetStorageKeyReadRefundUnits())
		readsRefundOp.MulAdd(uint64(v), r.GetStorageValueReadRefundUnits())
	}
	readRefunds, err := readsRefundOp.Value()
	if err != nil {
		return fees.Dimensions{}, err
	}

	return fees.Dimensions{0, 0, readRefunds, 0, 0}, nil
}
