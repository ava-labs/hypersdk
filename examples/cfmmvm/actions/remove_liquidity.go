package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/consts"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/storage"
	"github.com/ava-labs/hypersdk/state"

	smath "github.com/ava-labs/avalanchego/utils/math"
)

var _ chain.Action = (*RemoveLiquidity)(nil)

type RemoveLiquidity struct {
	LiquidityPool codec.Address `json:"liquidityPool"`
}

// ComputeUnits implements chain.Action.
func (r *RemoveLiquidity) ComputeUnits(chain.Rules) uint64 {
	return RemoveLiquidityComputeUnits
}

// Execute implements chain.Action.
func (l *RemoveLiquidity) Execute(ctx context.Context, r chain.Rules, mu state.Mutable, timestamp int64, actor codec.Address, actionID ids.ID) ([][]byte, error) {
	// Check that LP exists
	functionID, tokenX, tokenY, fee, reserveX, reserveY, lpTokenAddress, err := storage.GetLiquidityPoolNoController(ctx, mu, l.LiquidityPool)
	if err != nil {
		return nil, ErrOutputLiquidityPoolDoesNotExist
	}
	balanceX, err := storage.GetTokenAccountNoController(ctx, mu, tokenX, l.LiquidityPool)
	if err != nil {
		return nil, err
	}
	balanceY, err := storage.GetTokenAccountNoController(ctx, mu, tokenY, l.LiquidityPool)
	if err != nil {
		return nil, err
	}
	// Amount of LP tokens that were sent for burning
	currLPTokenBalance, err := storage.GetTokenAccountNoController(ctx, mu, lpTokenAddress, l.LiquidityPool)
	if err != nil {
		return nil, err
	}

	_, _, _, _, lpTokenSupply, _, err := storage.GetTokenInfoNoController(ctx, mu, lpTokenAddress)
	if err != nil {
		return nil, err
	}

	amountX, err := smath.Mul64(currLPTokenBalance, balanceX)
	if err != nil {
		return nil, err
	}
	amountX = amountX / lpTokenSupply
	amountY, err := smath.Mul64(currLPTokenBalance, balanceY)
	if err != nil {
		return nil, err
	}
	amountY = amountY / lpTokenSupply
	if amountX == 0 || amountY == 0 {
		return nil, ErrOutputInsufficientLiquidityBurned
	}
	// Burn LP token, transfer tokens X/Y to actor
	if err = storage.BurnToken(ctx, mu, lpTokenAddress, l.LiquidityPool, currLPTokenBalance); err != nil {
		return nil, err
	}
	if err = storage.TransferToken(ctx, mu, tokenX, l.LiquidityPool, actor, amountX); err != nil {
		return nil, err
	}
	if err = storage.TransferToken(ctx, mu, tokenY, l.LiquidityPool, actor, amountY); err != nil {
		return nil, err
	}
	newReserveX, err := smath.Sub(reserveX, amountX)
	if err != nil {
		return nil, err
	}
	newReserveY, err := smath.Sub(reserveY, amountY)
	if err != nil {
		return nil, err
	}
	// Finally, update LP pool
	if err = storage.SetLiquidityPool(ctx, mu, l.LiquidityPool, functionID, tokenX, tokenY, fee, newReserveX, newReserveY, lpTokenAddress); err != nil {
		return nil, err
	}

	return nil, nil
}

// GetTypeID implements chain.Action.
func (r *RemoveLiquidity) GetTypeID() uint8 {
	return consts.RemoveLiquidityID
}

// Size implements chain.Action.
func (r *RemoveLiquidity) Size() int {
	return codec.AddressLen
}

// StateKeys implements chain.Action.
func (r *RemoveLiquidity) StateKeys(actor codec.Address, actionID ids.ID) state.Keys {
	panic("unimplemented")
}

// StateKeysMaxChunks implements chain.Action.
func (r *RemoveLiquidity) StateKeysMaxChunks() []uint16 {
	panic("unimplemented")
}

// ValidRange implements chain.Action.
func (r *RemoveLiquidity) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

// Marshal implements chain.Action.
func (r *RemoveLiquidity) Marshal(p *codec.Packer) {
	p.PackAddress(r.LiquidityPool)
}

func UnmarshalRemoveLiquidity(p *codec.Packer) (chain.Action, error) {
	var removeLiquidity RemoveLiquidity
	p.UnpackAddress(&removeLiquidity.LiquidityPool)
	return &removeLiquidity, p.Err()
}