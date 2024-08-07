// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/chaintest"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/functions"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/tstate"

	smath "github.com/ava-labs/avalanchego/utils/math"
	lconsts "github.com/ava-labs/hypersdk/consts"
)

func TestCreateLiquidityPool(t *testing.T) {
	require := require.New(t)
	ts := tstate.New(1)

	onesAddr, err := createAddressWithSameDigits(1)
	require.NoError(err)

	createLiquidityPoolTests := []chaintest.ActionTest{
		{
			Name: "Invalid fee not allowed",
			Action: &CreateLiquidityPool{
				FunctionID: InitialFunctionID,
				TokenX:     tokenOneAddress,
				TokenY:     tokenTwoAddress,
				Fee:        0,
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputInvalidFee,
			State:           GenerateEmptyState(),
		},
		{
			Name: "Token X must exist",
			Action: &CreateLiquidityPool{
				FunctionID: InitialFunctionID,
				TokenX:     codec.EmptyAddress,
				TokenY:     tokenTwoAddress,
				Fee:        InitialFee,
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputTokenXDoesNotExist,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(storage.TokenInfoKey(tokenOneAddress)), state.Read)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
		},
		{
			Name: "Token Y must exist",
			Action: &CreateLiquidityPool{
				FunctionID: InitialFunctionID,
				TokenX:     tokenOneAddress,
				TokenY:     codec.EmptyAddress,
				Fee:        InitialFee,
			},
			SetupActions: []chain.Action{
				&CreateToken{
					Name:     []byte(TokenOneName),
					Symbol:   []byte(TokenOneSymbol),
					Metadata: []byte(TokenOneMetadata),
				},
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputTokenYDoesNotExist,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(storage.TokenInfoKey(tokenOneAddress)), state.All)
				stateKeys.Add(string(storage.TokenInfoKey(tokenTwoAddress)), state.Read)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
			Actor: onesAddr,
		},
		{
			Name: "No invalid constant function IDs",
			Action: &CreateLiquidityPool{
				FunctionID: functions.InvalidFormulaID,
				TokenX:     tokenOneAddress,
				TokenY:     tokenTwoAddress,
				Fee:        InitialFee,
			},
			SetupActions: []chain.Action{
				&CreateToken{
					Name:     []byte(TokenOneName),
					Symbol:   []byte(TokenOneSymbol),
					Metadata: []byte(TokenOneMetadata),
				},
				&CreateToken{
					Name:     []byte(TokenTwoName),
					Symbol:   []byte(TokenTwoSymbol),
					Metadata: []byte(TokenTwoMetadata),
				},
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputFunctionDoesNotExist,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(storage.TokenInfoKey(tokenOneAddress)), state.All)
				stateKeys.Add(string(storage.TokenInfoKey(tokenTwoAddress)), state.All)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
			Actor: onesAddr,
		},
		{
			Name: "Correct liquidity pool creations are allowed",
			Action: &CreateLiquidityPool{
				FunctionID: InitialFunctionID,
				TokenX:     tokenOneAddress,
				TokenY:     tokenTwoAddress,
				Fee:        InitialFee,
			},
			SetupActions: []chain.Action{
				&CreateToken{
					Name:     []byte(TokenOneName),
					Symbol:   []byte(TokenOneSymbol),
					Metadata: []byte(TokenOneMetadata),
				},
				&CreateToken{
					Name:     []byte(TokenTwoName),
					Symbol:   []byte(TokenTwoSymbol),
					Metadata: []byte(TokenTwoMetadata),
				},
			},
			ExpectedErr: nil,
			ExpectedOutputs: func() [][]byte {
				lpAddress, err := storage.LiquidityPoolAddress(tokenOneAddress, tokenTwoAddress)
				require.NoError(err)
				lpTokenAddress := storage.LiqudityPoolTokenAddress(lpAddress)
				return [][]byte{lpAddress[:], lpTokenAddress[:]}
			}(),
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				lpAddress, err := storage.LiquidityPoolAddress(tokenOneAddress, tokenTwoAddress)
				require.NoError(err)
				stateKeys.Add(string(storage.TokenInfoKey(tokenOneAddress)), state.All)
				stateKeys.Add(string(storage.TokenInfoKey(tokenTwoAddress)), state.All)
				stateKeys.Add(string(storage.TokenInfoKey(storage.LiqudityPoolTokenAddress(lpAddress))), state.All)
				stateKeys.Add(string(storage.LiquidityPoolKey(lpAddress)), state.All)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
			Assertion: func(m state.Mutable) bool {
				lpAddress, err := storage.LiquidityPoolAddress(tokenOneAddress, tokenTwoAddress)
				lpTokenAddress := storage.LiqudityPoolTokenAddress(lpAddress)
				require.NoError(err)
				functionID, tokenX, tokenY, fee, reserveX, reserveY, lpTokenAddressFromChain, err := storage.GetLiquidityPoolNoController(context.TODO(), m, lpAddress)
				require.NoError(err)
				functionsMatch := (InitialFunctionID == functionID)
				tokenXMatch := (tokenX == tokenOneAddress)
				tokenYMatch := (tokenY == tokenTwoAddress)
				feeMatch := (fee == InitialFee)
				reserveXMatch := (reserveX == 0)
				reserveYMatch := (reserveY == 0)
				lpTokenAddressMatch := (lpTokenAddressFromChain == lpTokenAddress)
				return functionsMatch && tokenXMatch && tokenYMatch && feeMatch && reserveXMatch && reserveYMatch && lpTokenAddressMatch
			},
			Actor: onesAddr,
		},
		{
			Name: "Duplicate pools are not allowed",
			Action: &CreateLiquidityPool{
				FunctionID: InitialFunctionID,
				TokenX:     tokenOneAddress,
				TokenY:     tokenTwoAddress,
				Fee:        InitialFee,
			},
			SetupActions: []chain.Action{
				&CreateToken{
					Name:     []byte(TokenOneName),
					Symbol:   []byte(TokenOneSymbol),
					Metadata: []byte(TokenOneMetadata),
				},
				&CreateToken{
					Name:     []byte(TokenTwoName),
					Symbol:   []byte(TokenTwoSymbol),
					Metadata: []byte(TokenTwoMetadata),
				},
				&CreateLiquidityPool{
					FunctionID: InitialFunctionID,
					TokenX:     tokenOneAddress,
					TokenY:     tokenTwoAddress,
					Fee:        InitialFee,
				},
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputLiquidityPoolAlreadyExists,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				lpAddress, err := storage.LiquidityPoolAddress(tokenOneAddress, tokenTwoAddress)
				require.NoError(err)
				stateKeys.Add(string(storage.TokenInfoKey(tokenOneAddress)), state.All)
				stateKeys.Add(string(storage.TokenInfoKey(tokenTwoAddress)), state.All)
				stateKeys.Add(string(storage.TokenInfoKey(storage.LiqudityPoolTokenAddress(lpAddress))), state.All)
				stateKeys.Add(string(storage.LiquidityPoolKey(lpAddress)), state.All)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
			Actor: onesAddr,
		},
		{
			Name: "TokenX cannot be the same as TokenY",
			Action: &CreateLiquidityPool{
				FunctionID: InitialFunctionID,
				TokenX:     tokenOneAddress,
				TokenY:     tokenOneAddress,
				Fee:        InitialFee,
			},
			SetupActions: []chain.Action{
				&CreateToken{
					Name:     []byte(TokenOneName),
					Symbol:   []byte(TokenOneSymbol),
					Metadata: []byte(TokenOneMetadata),
				},
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputIdenticalTokens,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(storage.TokenInfoKey(tokenOneAddress)), state.All)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
			Actor: onesAddr,
		},
	}

	chaintest.Run(t, createLiquidityPoolTests)
}

func TestDepositLiquidity(t *testing.T) {
	require := require.New(t)
	ts := tstate.New(1)

	onesAddr, err := createAddressWithSameDigits(1)
	require.NoError(err)

	lpAddress, err := storage.LiquidityPoolAddress(tokenOneAddress, tokenTwoAddress)
	require.NoError(err)

	lpTokenAddress := storage.LiqudityPoolTokenAddress(lpAddress)

	depositLiquidityTests := []chaintest.ActionTest{
		{
			Name: "Basic liquidity deposit works",
			Action: &DepositLiquidity{
				LiquidityPool: lpAddress,
			},
			SetupActions: []chain.Action{
				&CreateToken{
					Name:     []byte(TokenOneName),
					Symbol:   []byte(TokenOneSymbol),
					Metadata: []byte(TokenOneMetadata),
				},
				&CreateToken{
					Name:     []byte(TokenTwoName),
					Symbol:   []byte(TokenTwoSymbol),
					Metadata: []byte(TokenTwoMetadata),
				},
				&CreateLiquidityPool{
					FunctionID: InitialFunctionID,
					TokenX:     tokenOneAddress,
					TokenY:     tokenTwoAddress,
					Fee:        InitialFee,
				},
				&MintToken{
					To:    onesAddr,
					Value: 10_000,
					Token: tokenOneAddress,
				},
				&MintToken{
					To:    onesAddr,
					Value: 10_000,
					Token: tokenTwoAddress,
				},
				&TransferToken{
					To:           lpAddress,
					Value:        10_000,
					TokenAddress: tokenOneAddress,
				},
				&TransferToken{
					To:           lpAddress,
					Value:        10_000,
					TokenAddress: tokenTwoAddress,
				},
			},
			ExpectedErr:     nil,
			ExpectedOutputs: [][]byte(nil),
			Actor:           onesAddr,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(storage.TokenInfoKey(tokenOneAddress)), state.All)
				stateKeys.Add(string(storage.TokenInfoKey(tokenTwoAddress)), state.All)
				stateKeys.Add(string(storage.TokenInfoKey(lpTokenAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenOneAddress, lpAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenTwoAddress, lpAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenOneAddress, onesAddr)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenTwoAddress, onesAddr)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(lpTokenAddress, codec.EmptyAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(lpTokenAddress, onesAddr)), state.All)
				stateKeys.Add(string(storage.LiquidityPoolKey(lpAddress)), state.All)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
			Assertion: func(m state.Mutable) bool {
				functionID, tokenX, tokenY, fee, reserveX, reserveY, lpTokenAddressFromChain, err := storage.GetLiquidityPoolNoController(context.TODO(), m, lpAddress)
				require.NoError(err)
				_, _, _, tSupply, _, err := storage.GetTokenInfoNoController(context.Background(), m, lpTokenAddressFromChain)
				require.NoError(err)
				zeroAddressBalance, err := storage.GetTokenAccountNoController(context.TODO(), m, lpTokenAddressFromChain, codec.EmptyAddress)
				require.NoError(err)
				onesAddressBalance, err := storage.GetTokenAccountNoController(context.TODO(), m, lpTokenAddressFromChain, onesAddr)
				require.NoError(err)
				functionsMatch := (InitialFunctionID == functionID)
				tokenXMatch := (tokenX == tokenOneAddress)
				tokenYMatch := (tokenY == tokenTwoAddress)
				feeMatch := (fee == InitialFee)
				reserveXMatch := (reserveX == 10_000)
				reserveYMatch := (reserveY == 10_000)
				lpTokenAddressMatch := (lpTokenAddressFromChain == lpTokenAddress)
				totalSupplyMatch := (tSupply == 10_000)
				zeroAddressBalanceMatch := (zeroAddressBalance == 1_000)
				onesAddressBalanceMatch := (onesAddressBalance == 9_000)
				return functionsMatch && feeMatch && tokenXMatch && tokenYMatch && reserveXMatch && reserveYMatch && lpTokenAddressMatch && totalSupplyMatch && zeroAddressBalanceMatch && onesAddressBalanceMatch
			},
		},
		{
			Name: "LP must exist",
			Action: &DepositLiquidity{
				LiquidityPool: lpAddress,
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputLiquidityPoolDoesNotExist,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(storage.LiquidityPoolKey(lpAddress)), state.Read)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
		},
		{
			Name: "LP token must exist",
			Action: &DepositLiquidity{
				LiquidityPool: lpAddress,
			},
			SetupActions: []chain.Action{
				&CreateToken{
					Name:     []byte(TokenOneName),
					Symbol:   []byte(TokenOneSymbol),
					Metadata: []byte(TokenOneMetadata),
				},
				&CreateToken{
					Name:     []byte(TokenTwoName),
					Symbol:   []byte(TokenTwoSymbol),
					Metadata: []byte(TokenTwoMetadata),
				},
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputTokenDoesNotExist,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				mu := chaintest.NewInMemoryStore()
				stateKeys.Add(string(storage.TokenInfoKey(tokenOneAddress)), state.All)
				stateKeys.Add(string(storage.TokenInfoKey(tokenTwoAddress)), state.All)
				stateKeys.Add(string(storage.TokenInfoKey(lpTokenAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenOneAddress, lpAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenTwoAddress, lpAddress)), state.All)
				stateKeys.Add(string(storage.LiquidityPoolKey(lpAddress)), state.Read)
				require.NoError(storage.SetLiquidityPool(context.TODO(), mu, lpAddress, InitialFunctionID, tokenOneAddress, tokenTwoAddress, InitialFee, 0, 0, lpTokenAddress))
				return ts.NewView(stateKeys, mu.Storage)
			}(),
			Actor: onesAddr,
		},
		{
			Name: "Integer underflow when calculating amountX",
			Action: &DepositLiquidity{
				LiquidityPool: lpAddress,
			},
			SetupActions: []chain.Action{
				&CreateToken{
					Name:     []byte(TokenOneName),
					Symbol:   []byte(TokenOneSymbol),
					Metadata: []byte(TokenOneMetadata),
				},
				&CreateToken{
					Name:     []byte(TokenTwoName),
					Symbol:   []byte(TokenTwoSymbol),
					Metadata: []byte(TokenTwoMetadata),
				},
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     smath.ErrUnderflow,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				mu := chaintest.NewInMemoryStore()
				stateKeys.Add(string(storage.TokenInfoKey(tokenOneAddress)), state.All)
				stateKeys.Add(string(storage.TokenInfoKey(tokenTwoAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenOneAddress, lpAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenTwoAddress, lpAddress)), state.Read)
				stateKeys.Add(string(storage.LiquidityPoolKey(lpAddress)), state.Read)
				stateKeys.Add(string(storage.TokenInfoKey(lpTokenAddress)), state.Read)
				require.NoError(storage.SetTokenInfo(context.TODO(), mu, lpTokenAddress, []byte(storage.LiquidityPoolTokenName), []byte(storage.LiquidityPoolTokenSymbol), []byte(storage.LiquidityPoolTokenMetadata), 0, lpAddress))
				require.NoError(storage.SetLiquidityPool(context.TODO(), mu, lpAddress, InitialFunctionID, tokenOneAddress, tokenTwoAddress, InitialFee, 2, 2, lpTokenAddress))
				return ts.NewView(stateKeys, mu.Storage)
			}(),
			Actor: onesAddr,
		},
		{
			Name: "Integer underflow when calculating amountY",
			Action: &DepositLiquidity{
				LiquidityPool: lpAddress,
			},
			SetupActions: []chain.Action{
				&CreateToken{
					Name:     []byte(TokenOneName),
					Symbol:   []byte(TokenOneSymbol),
					Metadata: []byte(TokenOneMetadata),
				},
				&CreateToken{
					Name:     []byte(TokenTwoName),
					Symbol:   []byte(TokenTwoSymbol),
					Metadata: []byte(TokenTwoMetadata),
				},
				&MintToken{
					To:    lpAddress,
					Token: tokenOneAddress,
					Value: 2,
				},
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     smath.ErrUnderflow,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				mu := chaintest.NewInMemoryStore()
				stateKeys.Add(string(storage.TokenInfoKey(tokenOneAddress)), state.All)
				stateKeys.Add(string(storage.TokenInfoKey(tokenTwoAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenOneAddress, lpAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenTwoAddress, lpAddress)), state.All)
				stateKeys.Add(string(storage.LiquidityPoolKey(lpAddress)), state.Read)
				stateKeys.Add(string(storage.TokenInfoKey(lpTokenAddress)), state.Read)
				require.NoError(storage.SetTokenInfo(context.TODO(), mu, lpTokenAddress, []byte(storage.LiquidityPoolTokenName), []byte(storage.LiquidityPoolTokenSymbol), []byte(storage.LiquidityPoolTokenMetadata), 0, lpAddress))
				require.NoError(storage.SetLiquidityPool(context.TODO(), mu, lpAddress, InitialFunctionID, tokenOneAddress, tokenTwoAddress, InitialFee, 2, 2, lpTokenAddress))
				return ts.NewView(stateKeys, mu.Storage)
			}(),
			Actor: onesAddr,
		},
		{
			Name: "Correct instance of supplying liquidity to an operational (i.e. reserve != 0) pool",
			Action: &DepositLiquidity{
				LiquidityPool: lpAddress,
			},
			SetupActions: []chain.Action{
				&CreateToken{
					Name:     []byte(TokenOneName),
					Symbol:   []byte(TokenOneSymbol),
					Metadata: []byte(TokenOneMetadata),
				},
				&CreateToken{
					Name:     []byte(TokenTwoName),
					Symbol:   []byte(TokenTwoSymbol),
					Metadata: []byte(TokenTwoMetadata),
				},
				&CreateLiquidityPool{
					FunctionID: InitialFunctionID,
					TokenX:     tokenOneAddress,
					TokenY:     tokenTwoAddress,
					Fee:        InitialFee,
				},
				&MintToken{
					To:    onesAddr,
					Value: 15_000,
					Token: tokenOneAddress,
				},
				&MintToken{
					To:    onesAddr,
					Value: 15_000,
					Token: tokenTwoAddress,
				},
				&TransferToken{
					To:           lpAddress,
					Value:        10_000,
					TokenAddress: tokenOneAddress,
				},
				&TransferToken{
					To:           lpAddress,
					Value:        10_000,
					TokenAddress: tokenTwoAddress,
				},
				&DepositLiquidity{
					LiquidityPool: lpAddress,
				},
				&TransferToken{
					To:           lpAddress,
					Value:        5_000,
					TokenAddress: tokenOneAddress,
				},
				&TransferToken{
					To:           lpAddress,
					Value:        5_000,
					TokenAddress: tokenTwoAddress,
				},
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     nil,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(storage.TokenInfoKey(tokenOneAddress)), state.All)
				stateKeys.Add(string(storage.TokenInfoKey(tokenTwoAddress)), state.All)
				stateKeys.Add(string(storage.TokenInfoKey(lpTokenAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenOneAddress, lpAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenTwoAddress, lpAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenOneAddress, onesAddr)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenTwoAddress, onesAddr)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(lpTokenAddress, codec.EmptyAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(lpTokenAddress, onesAddr)), state.All)
				stateKeys.Add(string(storage.LiquidityPoolKey(lpAddress)), state.All)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
			Actor: onesAddr,
			Assertion: func(m state.Mutable) bool {
				onesAddrLPTokenBalance, err := storage.GetTokenAccountNoController(context.TODO(), m, lpTokenAddress, onesAddr)
				require.NoError(err)
				return onesAddrLPTokenBalance == 14_000
			},
		},
		{
			Name: "Liquidity must not equal 0",
			Action: &DepositLiquidity{
				LiquidityPool: lpAddress,
			},
			SetupActions: []chain.Action{
				&CreateToken{
					Name:     []byte(TokenOneName),
					Symbol:   []byte(TokenOneSymbol),
					Metadata: []byte(TokenOneMetadata),
				},
				&CreateToken{
					Name:     []byte(TokenTwoName),
					Symbol:   []byte(TokenTwoSymbol),
					Metadata: []byte(TokenTwoMetadata),
				},
				&CreateLiquidityPool{
					FunctionID: InitialFunctionID,
					TokenX:     tokenOneAddress,
					TokenY:     tokenTwoAddress,
					Fee:        InitialFee,
				},
				&MintToken{
					To:    onesAddr,
					Value: 15_000,
					Token: tokenOneAddress,
				},
				&MintToken{
					To:    onesAddr,
					Value: 15_000,
					Token: tokenTwoAddress,
				},
				&TransferToken{
					To:           lpAddress,
					Value:        10_000,
					TokenAddress: tokenOneAddress,
				},
				&TransferToken{
					To:           lpAddress,
					Value:        10_000,
					TokenAddress: tokenTwoAddress,
				},
				&DepositLiquidity{
					LiquidityPool: lpAddress,
				},
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputInsufficientLiquidityMinted,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(storage.TokenInfoKey(tokenOneAddress)), state.All)
				stateKeys.Add(string(storage.TokenInfoKey(tokenTwoAddress)), state.All)
				stateKeys.Add(string(storage.TokenInfoKey(lpTokenAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenOneAddress, lpAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenTwoAddress, lpAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenOneAddress, onesAddr)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenTwoAddress, onesAddr)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(lpTokenAddress, codec.EmptyAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(lpTokenAddress, onesAddr)), state.All)
				stateKeys.Add(string(storage.LiquidityPoolKey(lpAddress)), state.All)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
			Actor: onesAddr,
		},
		{
			Name: "Liquidity integer overflow (No Liquidity)",
			Action: &DepositLiquidity{
				LiquidityPool: lpAddress,
			},
			SetupActions: []chain.Action{
				&CreateToken{
					Name:     []byte(TokenOneName),
					Symbol:   []byte(TokenOneSymbol),
					Metadata: []byte(TokenOneMetadata),
				},
				&CreateToken{
					Name:     []byte(TokenTwoName),
					Symbol:   []byte(TokenTwoSymbol),
					Metadata: []byte(TokenTwoMetadata),
				},
				&CreateLiquidityPool{
					FunctionID: InitialFunctionID,
					TokenX:     tokenOneAddress,
					TokenY:     tokenTwoAddress,
					Fee:        InitialFee,
				},
				&MintToken{
					To:    onesAddr,
					Value: lconsts.MaxUint64,
					Token: tokenOneAddress,
				},
				&MintToken{
					To:    onesAddr,
					Value: lconsts.MaxUint64,
					Token: tokenTwoAddress,
				},
				&TransferToken{
					To:           lpAddress,
					Value:        lconsts.MaxUint64,
					TokenAddress: tokenOneAddress,
				},
				&TransferToken{
					To:           lpAddress,
					Value:        lconsts.MaxUint64,
					TokenAddress: tokenTwoAddress,
				},
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     smath.ErrOverflow,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(storage.TokenInfoKey(tokenOneAddress)), state.All)
				stateKeys.Add(string(storage.TokenInfoKey(tokenTwoAddress)), state.All)
				stateKeys.Add(string(storage.TokenInfoKey(lpTokenAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenOneAddress, lpAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenTwoAddress, lpAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenOneAddress, onesAddr)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenTwoAddress, onesAddr)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(lpTokenAddress, codec.EmptyAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(lpTokenAddress, onesAddr)), state.All)
				stateKeys.Add(string(storage.LiquidityPoolKey(lpAddress)), state.All)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
			Actor: onesAddr,
		},
		{
			Name: "Initial liquidity minted must be >= Minimum Liquidity",
			Action: &DepositLiquidity{
				LiquidityPool: lpAddress,
			},
			SetupActions: []chain.Action{
				&CreateToken{
					Name:     []byte(TokenOneName),
					Symbol:   []byte(TokenOneSymbol),
					Metadata: []byte(TokenOneMetadata),
				},
				&CreateToken{
					Name:     []byte(TokenTwoName),
					Symbol:   []byte(TokenTwoSymbol),
					Metadata: []byte(TokenTwoMetadata),
				},
				&CreateLiquidityPool{
					FunctionID: InitialFunctionID,
					TokenX:     tokenOneAddress,
					TokenY:     tokenTwoAddress,
					Fee:        InitialFee,
				},
				&MintToken{
					To:    onesAddr,
					Value: 15_000,
					Token: tokenOneAddress,
				},
				&MintToken{
					To:    onesAddr,
					Value: 15_000,
					Token: tokenTwoAddress,
				},
				&TransferToken{
					To:           lpAddress,
					Value:        1,
					TokenAddress: tokenOneAddress,
				},
				&TransferToken{
					To:           lpAddress,
					Value:        1,
					TokenAddress: tokenTwoAddress,
				},
			},
			ExpectedOutputs: nil,
			ExpectedErr:     smath.ErrUnderflow,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(storage.TokenInfoKey(tokenOneAddress)), state.All)
				stateKeys.Add(string(storage.TokenInfoKey(tokenTwoAddress)), state.All)
				stateKeys.Add(string(storage.TokenInfoKey(lpTokenAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenOneAddress, lpAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenTwoAddress, lpAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenOneAddress, onesAddr)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenTwoAddress, onesAddr)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(lpTokenAddress, codec.EmptyAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(lpTokenAddress, onesAddr)), state.All)
				stateKeys.Add(string(storage.LiquidityPoolKey(lpAddress)), state.All)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
			Actor: onesAddr,
		},
	}

	chaintest.Run(t, depositLiquidityTests)
}

func TestRemoveLiquidity(t *testing.T) {
	require := require.New(t)
	ts := tstate.New(1)

	onesAddr, err := createAddressWithSameDigits(1)
	require.NoError(err)

	lpAddress, err := storage.LiquidityPoolAddress(tokenOneAddress, tokenTwoAddress)
	require.NoError(err)

	lpTokenAddress := storage.LiqudityPoolTokenAddress(lpAddress)

	removeLiquidityTests := []chaintest.ActionTest{
		{
			Name: "LP must exist",
			Action: &RemoveLiquidity{
				LiquidityPool: lpAddress,
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputLiquidityPoolDoesNotExist,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(storage.LiquidityPoolKey(lpAddress)), state.Read)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
		},
		{
			Name: "Basic liquidity removal",
			Action: &RemoveLiquidity{
				LiquidityPool: lpAddress,
			},
			SetupActions: []chain.Action{
				&CreateToken{
					Name:     []byte(TokenOneName),
					Symbol:   []byte(TokenOneSymbol),
					Metadata: []byte(TokenOneMetadata),
				},
				&CreateToken{
					Name:     []byte(TokenTwoName),
					Symbol:   []byte(TokenTwoSymbol),
					Metadata: []byte(TokenTwoMetadata),
				},
				&CreateLiquidityPool{
					FunctionID: InitialFunctionID,
					TokenX:     tokenOneAddress,
					TokenY:     tokenTwoAddress,
					Fee:        InitialFee,
				},
				&MintToken{
					To:    onesAddr,
					Value: 10_000,
					Token: tokenOneAddress,
				},
				&MintToken{
					To:    onesAddr,
					Value: 10_000,
					Token: tokenTwoAddress,
				},
				&TransferToken{
					To:           lpAddress,
					Value:        10_000,
					TokenAddress: tokenOneAddress,
				},
				&TransferToken{
					To:           lpAddress,
					Value:        10_000,
					TokenAddress: tokenTwoAddress,
				},
				&DepositLiquidity{
					LiquidityPool: lpAddress,
				},
				&TransferToken{
					To:           lpAddress,
					Value:        1_000,
					TokenAddress: lpTokenAddress,
				},
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     nil,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(storage.TokenInfoKey(tokenOneAddress)), state.All)
				stateKeys.Add(string(storage.TokenInfoKey(tokenTwoAddress)), state.All)
				stateKeys.Add(string(storage.TokenInfoKey(lpTokenAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenOneAddress, lpAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenTwoAddress, lpAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenOneAddress, onesAddr)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenTwoAddress, onesAddr)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(lpTokenAddress, codec.EmptyAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(lpTokenAddress, onesAddr)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(lpTokenAddress, lpAddress)), state.All)
				stateKeys.Add(string(storage.LiquidityPoolKey(lpAddress)), state.All)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
			Actor: onesAddr,
			Assertion: func(m state.Mutable) bool {
				tokenOneBalance, err := storage.GetTokenAccountNoController(context.TODO(), m, tokenOneAddress, onesAddr)
				require.NoError(err)
				tokenTwoBalance, err := storage.GetTokenAccountNoController(context.TODO(), m, tokenTwoAddress, onesAddr)
				require.NoError(err)
				return tokenOneBalance == 1_000 && tokenTwoBalance == 1_000
			},
		},
		// {
		// 	Name: "Sufficient liquidity must be burned",
		// },
	}

	chaintest.Run(t, removeLiquidityTests)
}

// Tests are generated using the following:
// https://www.desmos.com/calculator/pn5fzoau2f
func TestSwap(t *testing.T) {
	require := require.New(t)
	ts := tstate.New(1)

	onesAddr, err := createAddressWithSameDigits(1)
	require.NoError(err)

	lpAddress, err := storage.LiquidityPoolAddress(tokenOneAddress, tokenTwoAddress)
	require.NoError(err)

	lpTokenAddress := storage.LiqudityPoolTokenAddress(lpAddress)

	swapTests := []chaintest.ActionTest{
		{
			Name: "Basic swap",
			Action: &Swap{
				AmountXIn: InitialSwapValue,
				AmountYIn: 0,
				LPAddress: lpAddress,
			},
			SetupActions: []chain.Action{
				&CreateToken{
					Name:     []byte(TokenOneName),
					Symbol:   []byte(TokenOneSymbol),
					Metadata: []byte(TokenOneMetadata),
				},
				&CreateToken{
					Name:     []byte(TokenTwoName),
					Symbol:   []byte(TokenTwoSymbol),
					Metadata: []byte(TokenTwoMetadata),
				},
				&CreateLiquidityPool{
					FunctionID: InitialFunctionID,
					TokenX:     tokenOneAddress,
					TokenY:     tokenTwoAddress,
					Fee:        InitialFee,
				},
				&MintToken{
					To:    onesAddr,
					Value: 10_000 + InitialSwapValue,
					Token: tokenOneAddress,
				},
				&MintToken{
					To:    onesAddr,
					Value: 10_000,
					Token: tokenTwoAddress,
				},
				&TransferToken{
					To:           lpAddress,
					Value:        10_000,
					TokenAddress: tokenOneAddress,
				},
				&TransferToken{
					To:           lpAddress,
					Value:        10_000,
					TokenAddress: tokenTwoAddress,
				},
				&DepositLiquidity{
					LiquidityPool: lpAddress,
				},
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     nil,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(storage.TokenInfoKey(tokenOneAddress)), state.All)
				stateKeys.Add(string(storage.TokenInfoKey(tokenTwoAddress)), state.All)
				stateKeys.Add(string(storage.TokenInfoKey(lpTokenAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenOneAddress, lpAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenTwoAddress, lpAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenOneAddress, onesAddr)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenTwoAddress, onesAddr)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(lpTokenAddress, codec.EmptyAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(lpTokenAddress, onesAddr)), state.All)
				stateKeys.Add(string(storage.LiquidityPoolKey(lpAddress)), state.All)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
			Assertion: func(m state.Mutable) bool {
				balance, err := storage.GetTokenAccountNoController(context.TODO(), m, tokenTwoAddress, onesAddr)
				require.NoError(err)
				return balance == 100
			},
			Actor: onesAddr,
		},
		{
			Name: "Basic swap (in reverse direction)",
			Action: &Swap{
				AmountXIn: 0,
				AmountYIn: InitialSwapValue,
				LPAddress: lpAddress,
			},
			SetupActions: []chain.Action{
				&CreateToken{
					Name:     []byte(TokenOneName),
					Symbol:   []byte(TokenOneSymbol),
					Metadata: []byte(TokenOneMetadata),
				},
				&CreateToken{
					Name:     []byte(TokenTwoName),
					Symbol:   []byte(TokenTwoSymbol),
					Metadata: []byte(TokenTwoMetadata),
				},
				&CreateLiquidityPool{
					FunctionID: InitialFunctionID,
					TokenX:     tokenOneAddress,
					TokenY:     tokenTwoAddress,
					Fee:        InitialFee,
				},
				&MintToken{
					To:    onesAddr,
					Value: 10_000,
					Token: tokenOneAddress,
				},
				&MintToken{
					To:    onesAddr,
					Value: 10_000 + InitialSwapValue,
					Token: tokenTwoAddress,
				},
				&TransferToken{
					To:           lpAddress,
					Value:        10_000,
					TokenAddress: tokenOneAddress,
				},
				&TransferToken{
					To:           lpAddress,
					Value:        10_000,
					TokenAddress: tokenTwoAddress,
				},
				&DepositLiquidity{
					LiquidityPool: lpAddress,
				},
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     nil,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(storage.TokenInfoKey(tokenOneAddress)), state.All)
				stateKeys.Add(string(storage.TokenInfoKey(tokenTwoAddress)), state.All)
				stateKeys.Add(string(storage.TokenInfoKey(lpTokenAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenOneAddress, lpAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenTwoAddress, lpAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenOneAddress, onesAddr)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenTwoAddress, onesAddr)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(lpTokenAddress, codec.EmptyAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(lpTokenAddress, onesAddr)), state.All)
				stateKeys.Add(string(storage.LiquidityPoolKey(lpAddress)), state.All)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
			Assertion: func(m state.Mutable) bool {
				balance, err := storage.GetTokenAccountNoController(context.TODO(), m, tokenOneAddress, onesAddr)
				require.NoError(err)
				return balance == 100
			},
			Actor: onesAddr,
		},
		{
			Name: "LP must exist",
			Action: &Swap{
				AmountXIn: InitialSwapValue,
				AmountYIn: 0,
				LPAddress: lpAddress,
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputLiquidityPoolDoesNotExist,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(storage.LiquidityPoolKey(lpAddress)), state.Read)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
		},
		{
			Name: "Function must exist",
			Action: &Swap{
				AmountXIn: InitialSwapValue,
				AmountYIn: 0,
				LPAddress: lpAddress,
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputFunctionDoesNotExist,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(storage.LiquidityPoolKey(lpAddress)), state.Read)
				mu := chaintest.NewInMemoryStore()
				require.NoError(storage.SetLiquidityPool(context.TODO(), mu, lpAddress, functions.InvalidFormulaID, tokenOneAddress, tokenTwoAddress, InitialFee, 0, 0, lpTokenAddress))
				return ts.NewView(stateKeys, mu.Storage)
			}(),
		},
		{
			Name: "Both deltas cannot be zero",
			Action: &Swap{
				AmountXIn: 0,
				AmountYIn: 0,
				LPAddress: lpAddress,
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     functions.ErrBothDeltasZero,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				mu := chaintest.NewInMemoryStore()
				stateKeys.Add(string(storage.LiquidityPoolKey(lpAddress)), state.Read)
				require.NoError(storage.SetLiquidityPool(context.TODO(), mu, lpAddress, InitialFunctionID, tokenOneAddress, tokenTwoAddress, InitialFee, 1, 1, lpTokenAddress))
				return ts.NewView(stateKeys, mu.Storage)
			}(),
		},
		{
			Name: "Both deltas cannot be nonzero",
			Action: &Swap{
				AmountXIn: 1,
				AmountYIn: 1,
				LPAddress: lpAddress,
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     functions.ErrNoClearDeltaToCompute,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				mu := chaintest.NewInMemoryStore()
				stateKeys.Add(string(storage.LiquidityPoolKey(lpAddress)), state.Read)
				require.NoError(storage.SetLiquidityPool(context.TODO(), mu, lpAddress, InitialFunctionID, tokenOneAddress, tokenTwoAddress, InitialFee, 1, 1, lpTokenAddress))
				return ts.NewView(stateKeys, mu.Storage)
			}(),
		},
		{
			Name: "Reserves cannot be zero",
			Action: &Swap{
				AmountXIn: InitialSwapValue,
				AmountYIn: 0,
				LPAddress: lpAddress,
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     functions.ErrReservesZero,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				mu := chaintest.NewInMemoryStore()
				stateKeys.Add(string(storage.LiquidityPoolKey(lpAddress)), state.Read)
				require.NoError(storage.SetLiquidityPool(context.TODO(), mu, lpAddress, InitialFunctionID, tokenOneAddress, tokenTwoAddress, InitialFee, 0, 0, lpTokenAddress))
				return ts.NewView(stateKeys, mu.Storage)
			}(),
		},
	}

	chaintest.Run(t, swapTests)
}
