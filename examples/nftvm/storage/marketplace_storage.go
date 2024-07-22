// Contains read/write logic pertaining to the NFT marketplace
package storage

import (
	"context"
	"encoding/binary"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/hashing"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/state"
)

const MarketplaceOrderStateChunks = consts.Uint64Len

// TODO: tune this
const BuyNFTStateChunks = 50

func MarketplaceOrderStateKey(
	orderID ids.ID,
) (k []byte) {
	k = make([]byte, consts.Uint16Len+ids.IDLen+consts.Uint16Len)
	k[0] = marketplaceOrderPrefix
	copy(k[1:], orderID[:])
	binary.BigEndian.PutUint16(k[1+ids.IDLen:], MarketplaceOrderStateChunks)
	return
}

func GenerateOrderID(
	parentCollection codec.Address,
	instanceNum uint32,
	price uint64,
) (ids.ID, error) {
	v := make([]byte, codec.AddressLen+consts.Uint32Len+consts.Uint64Len)
	// TODO: Check that this is OK
	id := hashing.ComputeHash256Array(v)
	return id, nil
}

func CreateMarketplaceOrder(
	ctx context.Context,
	mu state.Mutable,
	parentCollection codec.Address,
	instanceNum uint32,
	price uint64,
) (orderID ids.ID, err error) {
	// Generate orderID + state key
	orderID, err = GenerateOrderID(parentCollection, instanceNum, price)
	if err != nil {
		return ids.Empty, err
	}
	orderStateKey := MarketplaceOrderStateKey(orderID)
	// Check that order does not already exist
	if _, err := mu.GetValue(ctx, orderStateKey); err == nil {
		return ids.Empty, ErrOrderAlreadyExists
	}

	v := make([]byte, consts.Uint64Len)
	binary.BigEndian.PutUint64(v, price)

	// Order is new, complete execution
	return orderID, mu.Insert(ctx, orderStateKey, v)
}

func GetMarketplaceOrder(
	ctx context.Context,
	f ReadState,
	orderID ids.ID,
) (price uint64, err error) {
	k := MarketplaceOrderStateKey(orderID)
	values, errs := f(ctx, [][]byte{k})
	return innerGetMarketplaceOrder(values[0], errs[0])
}

func GetMarketplaceOrderNoController(
	ctx context.Context,
	mu state.Mutable,
	orderID ids.ID,
) (price uint64, err error) {
	k := MarketplaceOrderStateKey(orderID)
	v, err := mu.GetValue(ctx, k)
	return innerGetMarketplaceOrder(v, err)
}

func innerGetMarketplaceOrder(
	v []byte,
	e error,
) (price uint64, err error) {
	if e != nil {
		return 0, e
	}
	return binary.BigEndian.Uint64(v), nil
}
