package test

import (
	"context"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
	"github.com/near/borsh-go"
	"github.com/stretchr/testify/require"
	"testing"
)

const aLottaFuel = uint64(99999999999)

func TestTokenAMM(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	token1ID := ids.GenerateTestID()
	token1Address := codec.CreateAddress(1, token1ID)
	token2ID := ids.GenerateTestID()
	token2Address := codec.CreateAddress(1, token2ID)
	ammID := ids.GenerateTestID()
	ammAddress := codec.CreateAddress(1, ammID)

	token1Owner := codec.CreateAddress(2, ids.GenerateTestID())
	token2Owner := codec.CreateAddress(2, ids.GenerateTestID())
	ammOwner := codec.CreateAddress(2, ids.GenerateTestID())
	randomActor := codec.CreateAddress(2, ids.GenerateTestID())
	r := runtime.NewRuntime(
		runtime.NewConfig(),
		logging.NoLog{},
		MapLoader{programs: map[ids.ID]string{token1ID: "erc20_token", token2ID: "erc20_token", ammID: "amm"}},
	)

	state := NewTestDB()

	// init both tokens

	paramBytes, err := borsh.Serialize(struct {
		Name   string
		Symbol string
	}{
		Name:   "Token 1",
		Symbol: "TKN1",
	})
	require.NoError(err)
	_, err = r.CallProgram(ctx, &runtime.CallInfo{
		ProgramID:    token1ID,
		Actor:        token1Owner,
		Account:      token1Address,
		State:        state,
		FunctionName: "init",
		Params:       paramBytes,
		Fuel:         aLottaFuel})
	require.NoError(err)

	paramBytes, err = borsh.Serialize(struct {
		Name   string
		Symbol string
	}{
		Name:   "Token 2",
		Symbol: "TKN2",
	})
	_, err = r.CallProgram(ctx, &runtime.CallInfo{
		ProgramID:    token2ID,
		Actor:        token2Owner,
		Account:      token2Address,
		State:        state,
		FunctionName: "init",
		Params:       paramBytes,
		Fuel:         aLottaFuel})
	require.NoError(err)

	// init amm
	paramBytes, err = borsh.Serialize(struct {
		Token1 ids.ID
		Token2 ids.ID
	}{
		Token1: token1ID,
		Token2: token2ID,
	})
	_, err = r.CallProgram(ctx, &runtime.CallInfo{
		ProgramID:    ammID,
		Actor:        ammOwner,
		Account:      ammAddress,
		State:        state,
		FunctionName: "init",
		Params:       paramBytes,
		Fuel:         aLottaFuel})
	require.NoError(err)

	// mint tokens
	paramBytes, err = borsh.Serialize(struct {
		Recipient codec.Address
		Amount    uint64
	}{
		Recipient: randomActor,
		Amount:    10,
	})
	_, err = r.CallProgram(ctx, &runtime.CallInfo{
		ProgramID:    token1ID,
		Actor:        token1Owner,
		Account:      token1Address,
		State:        state,
		FunctionName: "mint",
		Params:       paramBytes,
		Fuel:         aLottaFuel})
	require.NoError(err)

	paramBytes, err = borsh.Serialize(struct {
		Recipient codec.Address
		Amount    uint64
	}{
		Recipient: randomActor,
		Amount:    10,
	})
	_, err = r.CallProgram(ctx, &runtime.CallInfo{
		ProgramID:    token2ID,
		Actor:        token2Owner,
		Account:      token2Address,
		State:        state,
		FunctionName: "mint",
		Params:       paramBytes,
		Fuel:         aLottaFuel})
	require.NoError(err)

	// approve token transfers
	paramBytes, err = borsh.Serialize(struct {
		Spender codec.Address
		Amount  uint64
	}{
		Spender: ammAddress,
		Amount:  10,
	})

	_, err = r.CallProgram(ctx, &runtime.CallInfo{
		ProgramID:    token1ID,
		Actor:        randomActor,
		Account:      token1Address,
		State:        state,
		FunctionName: "approve",
		Params:       paramBytes,
		Fuel:         aLottaFuel})
	require.NoError(err)

	_, err = r.CallProgram(ctx, &runtime.CallInfo{
		ProgramID:    token2ID,
		Actor:        randomActor,
		Account:      token2Address,
		State:        state,
		FunctionName: "approve",
		Params:       paramBytes,
		Fuel:         aLottaFuel})
	require.NoError(err)

	/*
		// test approval
		paramBytes, err = borsh.Serialize(struct {
			Spender   codec.Address
			Recipient codec.Address
			Amount    uint64
		}{
			Spender:   randomActor,
			Recipient: ammAddress,
			Amount:    5,
		})

		_, err = r.CallProgram(ctx, &runtime.CallInfo{
			ProgramID:    token1ID,
			Actor:        ammAddress,
			Account:      token1Address,
			State:        state,
			FunctionName: "transfer_from",
			Params:       paramBytes,
			Fuel:         aLottaFuel})
		require.NoError(err)

		// test approval
		paramBytes, err = borsh.Serialize(struct {
			Spender codec.Address
		}{
			Spender: ammAddress,
		})

		result, err := r.CallProgram(ctx, &runtime.CallInfo{
			ProgramID:    token1ID,
			Actor:        ammAddress,
			Account:      token1Address,
			State:        state,
			FunctionName: "balance_of",
			Params:       paramBytes,
			Fuel:         aLottaFuel})
		require.NoError(err)
		var amountOfToken1 uint64

		err = borsh.Deserialize(&amountOfToken1, result)
		require.Equal(uint64(5), amountOfToken1)
	*/

	// add liquidity
	paramBytes, err = borsh.Serialize(struct {
		Amount1 uint64
		Amount2 uint64
	}{
		Amount1: 5,
		Amount2: 5,
	})

	_, err = r.CallProgram(ctx, &runtime.CallInfo{
		ProgramID:    ammID,
		Actor:        randomActor,
		Account:      ammAddress,
		State:        state,
		FunctionName: "add_liquidity",
		Params:       paramBytes,
		Fuel:         aLottaFuel})
	require.NoError(err)

	// confirm liquidity
	paramBytes, err = borsh.Serialize(struct {
		Address codec.Address
	}{
		Address: randomActor,
	})

	result, err := r.CallProgram(ctx, &runtime.CallInfo{
		ProgramID:    ammID,
		Actor:        randomActor,
		Account:      ammAddress,
		State:        state,
		FunctionName: "balance_of",
		Params:       paramBytes,
		Fuel:         aLottaFuel})
	require.NoError(err)
	var amountOfLiquidity uint64

	err = borsh.Deserialize(&amountOfLiquidity, result)
	require.Equal(uint64(10), amountOfLiquidity)

}
