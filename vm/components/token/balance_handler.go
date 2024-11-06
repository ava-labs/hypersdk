package token

import (
	"context"
	"fmt"

	smath "github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/metadata"
)

const (
	BalanceChunks uint16 = 1
	BalanceHandlerPrefix byte = metadata.DefaultMinimumPrefix 
)

var _ (chain.BalanceHandler) = (*BalanceHandler)(nil)

type BalanceHandler struct{
	prefix []byte
}

func NewBalanceHandler(prefix []byte) *BalanceHandler {
	return &BalanceHandler{
		prefix: append([]byte{BalanceHandlerPrefix}, prefix...),
	}
}

func (bh *BalanceHandler) SponsorStateKeys(addr codec.Address) state.Keys {
	return state.Keys{
		string(bh.BalanceKey(addr)): state.Read | state.Write,
	}
}

func (bh *BalanceHandler) CanDeduct(
	ctx context.Context,
	addr codec.Address,
	im state.Immutable,
	amount uint64,
) error {
	bal, err := bh.GetBalance(ctx, im, addr)
	if err != nil {
		return err
	}
	if bal < amount {
		return ErrInvalidBalance
	}
	return nil
}

func (bh *BalanceHandler) Deduct(
	ctx context.Context,
	addr codec.Address,
	mu state.Mutable,
	amount uint64,
) error {
	_, err := bh.SubBalance(ctx, mu, addr, amount)
	return err
}

func (bh *BalanceHandler) AddBalance(
	ctx context.Context,
	addr codec.Address,
	mu state.Mutable,
	amount uint64,
	createAccount bool,
) error {
	key, bal, exists, err := bh.getBalance(ctx, mu, addr)
	if err != nil {
		return nil
	}
	// Don't add balance if account doesn't exist. This
	// can be useful when processing fee refunds.
	if !exists && !createAccount {
		return nil
	}
	nbal, err := smath.Add(bal, amount)
	if err != nil {
		return fmt.Errorf(
			"%w: could not add balance (bal=%d, addr=%v, amount=%d)",
			ErrInvalidBalance,
			bal,
			addr,
			amount,
		)
	}
	return setBalance(ctx, mu, key, nbal)
}

func (bh *BalanceHandler) GetBalance(ctx context.Context, im state.Immutable, addr codec.Address) (uint64, error) {
	_, bal, _, err := bh.getBalance(ctx, im, addr)
	return bal, err
}
