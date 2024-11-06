package token

import (
	"context"
	"fmt"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	smath "github.com/ava-labs/avalanchego/utils/math"

)

func AddBalance(
	ctx context.Context,
	bh *BalanceHandler,
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
