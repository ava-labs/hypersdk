package runtime

import (
	"context"
	"github.com/ava-labs/hypersdk/x/programs/test"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/near/borsh-go"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
)

const aLottaFuel = uint64(99999999999)

func TestTokenAMM(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stateDB := test.NewTestDB()

	token1ID := ids.GenerateTestID()
	token1 := ProgramInfo{ID: token1ID, Account: codec.CreateAddress(1, token1ID)}
	token1Owner := codec.CreateAddress(1, ids.GenerateTestID())

	token2ID := ids.GenerateTestID()
	token2 := ProgramInfo{ID: token2ID, Account: codec.CreateAddress(2, token2ID)}
	token2Owner := codec.CreateAddress(2, ids.GenerateTestID())

	ammID := ids.GenerateTestID()
	amm := ProgramInfo{ID: ammID, Account: codec.CreateAddress(3, ammID)}
	ammOwner := codec.CreateAddress(3, ids.GenerateTestID())

	r := NewRuntime(
		NewConfig(),
		logging.NoLog{},
		test.MapLoader{Programs: map[ids.ID]string{token1ID: "erc20_token", token2ID: "erc20_token", ammID: "amm"}},
	)

	// init token 1
	paramBytes, err := borsh.Serialize(struct {
		Name   string
		Symbol string
	}{
		Name:   "Token 1",
		Symbol: "TKN1",
	})
	require.NoError(err)
	_, err = r.CallProgram(ctx, &CallInfo{
		Program:      token1,
		Actor:        token1Owner,
		State:        stateDB,
		FunctionName: "init",
		Params:       paramBytes,
		Fuel:         aLottaFuel,
	})
	require.NoError(err)

	// init token 2
	paramBytes, err = borsh.Serialize(struct {
		Name   string
		Symbol string
	}{
		Name:   "Token 2",
		Symbol: "TKN2",
	})
	require.NoError(err)
	_, err = r.CallProgram(ctx, &CallInfo{
		Program:      token2,
		Actor:        token2Owner,
		State:        stateDB,
		FunctionName: "init",
		Params:       paramBytes,
		Fuel:         aLottaFuel,
	})
	require.NoError(err)

	// init amm
	paramBytes, err = borsh.Serialize(struct {
		Token1 ProgramInfo
		Token2 ProgramInfo
	}{
		Token1: token1,
		Token2: token2,
	})
	_, err = r.CallProgram(ctx, &CallInfo{
		Program:      amm,
		Actor:        ammOwner,
		State:        stateDB,
		FunctionName: "init",
		Params:       paramBytes,
		Fuel:         aLottaFuel,
	})
	require.NoError(err)

	user1 := codec.CreateAddress(8, ids.GenerateTestID())
	setupUser(ctx, require, r, stateDB, token1, token1Owner, user1, amm.Account, 5)
	setupUser(ctx, require, r, stateDB, token2, token2Owner, user1, amm.Account, 5)
	AddLiquitityToUser(ctx, require, r, stateDB, user1, token1, token2, amm, 5)

	result, err := r.CallProgram(ctx, &CallInfo{
		Program:      amm,
		Actor:        user1,
		State:        stateDB,
		FunctionName: "total_supply",
		Params:       nil,
		Fuel:         aLottaFuel,
	})
	require.NoError(err)
	var amountOfLiquidity uint64
	require.NoError(borsh.Deserialize(&amountOfLiquidity, result))
	require.Equal(uint64(10), amountOfLiquidity)

	user2 := codec.CreateAddress(9, ids.GenerateTestID())
	setupUser(ctx, require, r, stateDB, token1, token1Owner, user2, amm.Account, 6)
	setupUser(ctx, require, r, stateDB, token2, token2Owner, user2, amm.Account, 6)
	AddLiquitityToUser(ctx, require, r, stateDB, user2, token1, token2, amm, 6)

	result, err = r.CallProgram(ctx, &CallInfo{
		Program:      amm,
		Actor:        user1,
		State:        stateDB,
		FunctionName: "total_supply",
		Params:       nil,
		Fuel:         aLottaFuel,
	})
	require.NoError(err)
	require.NoError(borsh.Deserialize(&amountOfLiquidity, result))
	require.Equal(uint64(22), amountOfLiquidity)

	RemoveLiquidityFromUser(ctx, require, r, stateDB, user1, token1, token2, amm, 10)
	result, err = r.CallProgram(ctx, &CallInfo{
		Program:      amm,
		Actor:        user1,
		State:        stateDB,
		FunctionName: "total_supply",
		Params:       nil,
		Fuel:         aLottaFuel,
	})
	require.NoError(err)
	require.NoError(borsh.Deserialize(&amountOfLiquidity, result))
	require.Equal(uint64(12), amountOfLiquidity)

	RemoveLiquidityFromUser(ctx, require, r, stateDB, user2, token1, token2, amm, 12)

	result, err = r.CallProgram(ctx, &CallInfo{
		Program:      amm,
		Actor:        user1,
		State:        stateDB,
		FunctionName: "total_supply",
		Params:       nil,
		Fuel:         aLottaFuel,
	})
	require.NoError(err)
	require.NoError(borsh.Deserialize(&amountOfLiquidity, result))
	require.Equal(uint64(0), amountOfLiquidity)
}

func RemoveLiquidityFromUser(ctx context.Context, require *require.Assertions, r *WasmRuntime, stateDB state.Mutable, actor codec.Address, token1 ProgramInfo, token2 ProgramInfo, amm ProgramInfo, amount uint64) {
	paramBytes, err := borsh.Serialize(struct {
		Amount uint64
	}{
		Amount: amount,
	})

	_, err = r.CallProgram(ctx, &CallInfo{
		Program:      amm,
		Actor:        actor,
		State:        stateDB,
		FunctionName: "remove_liquidity",
		Params:       paramBytes,
		Fuel:         aLottaFuel,
	})
	require.NoError(err)

	paramBytes, err = borsh.Serialize(struct {
		Address codec.Address
	}{
		Address: actor,
	})
	require.NoError(err)

	result, err := r.CallProgram(ctx, &CallInfo{
		Program:      amm,
		Actor:        actor,
		State:        stateDB,
		FunctionName: "balance_of",
		Params:       paramBytes,
		Fuel:         aLottaFuel,
	})
	require.NoError(err)
	var amountOfLiquidity uint64
	require.NoError(borsh.Deserialize(&amountOfLiquidity, result))
	require.Equal(uint64(0), amountOfLiquidity)

	// confirm balance change for user1 in token 1
	result, err = r.CallProgram(ctx, &CallInfo{
		Program:      token1,
		Actor:        actor,
		State:        stateDB,
		FunctionName: "balance_of",
		Params:       paramBytes,
		Fuel:         aLottaFuel,
	})
	require.NoError(err)
	var amountOftoken1 uint64
	require.NoError(borsh.Deserialize(&amountOftoken1, result))
	require.Equal(amount/2, amountOftoken1)

	// confirm balance change for user1 in token 1
	result, err = r.CallProgram(ctx, &CallInfo{
		Program:      token2,
		Actor:        actor,
		State:        stateDB,
		FunctionName: "balance_of",
		Params:       paramBytes,
		Fuel:         aLottaFuel,
	})
	require.NoError(err)
	var amountOftoken2 uint64
	require.NoError(borsh.Deserialize(&amountOftoken2, result))
	require.Equal(amount/2, amountOftoken2)
}

func AddLiquitityToUser(ctx context.Context, require *require.Assertions, r *WasmRuntime, stateDB state.Mutable, actor codec.Address, token1 ProgramInfo, token2 ProgramInfo, amm ProgramInfo, amount uint64) {
	paramBytes, err := borsh.Serialize(struct {
		Amount1 uint64
		Amount2 uint64
	}{
		Amount1: amount,
		Amount2: amount,
	})

	_, err = r.CallProgram(ctx, &CallInfo{
		Program:      amm,
		Actor:        actor,
		State:        stateDB,
		FunctionName: "add_liquidity",
		Params:       paramBytes,
		Fuel:         aLottaFuel,
	})
	require.NoError(err)

	// confirm liquidity
	paramBytes, err = borsh.Serialize(struct {
		Address codec.Address
	}{
		Address: actor,
	})

	result, err := r.CallProgram(ctx, &CallInfo{
		Program:      amm,
		Actor:        actor,
		State:        stateDB,
		FunctionName: "balance_of",
		Params:       paramBytes,
		Fuel:         aLottaFuel,
	})
	require.NoError(err)
	var amountOfLiquidity uint64
	require.NoError(borsh.Deserialize(&amountOfLiquidity, result))
	require.Equal(amount*2, amountOfLiquidity)

	// confirm balance change for user1 in token 1
	result, err = r.CallProgram(ctx, &CallInfo{
		Program:      token1,
		Actor:        actor,
		State:        stateDB,
		FunctionName: "balance_of",
		Params:       paramBytes,
		Fuel:         aLottaFuel,
	})
	require.NoError(err)
	var amountOftoken1 uint64
	require.NoError(borsh.Deserialize(&amountOftoken1, result))
	require.Equal(uint64(0), amountOftoken1)

	// confirm balance change for user1 in token 1
	result, err = r.CallProgram(ctx, &CallInfo{
		Program:      token2,
		Actor:        actor,
		State:        stateDB,
		FunctionName: "balance_of",
		Params:       paramBytes,
		Fuel:         aLottaFuel,
	})
	require.NoError(err)
	var amountOftoken2 uint64
	require.NoError(borsh.Deserialize(&amountOftoken2, result))
	require.Equal(uint64(0), amountOftoken2)
}

func setupUser(ctx context.Context, require *require.Assertions, r *WasmRuntime, state state.Mutable, token ProgramInfo, owner codec.Address, actor codec.Address, ammAddress codec.Address, amount uint64) {
	paramBytes, err := borsh.Serialize(struct {
		Recipient codec.Address
		Amount    uint64
	}{
		Recipient: actor,
		Amount:    amount,
	})
	_, err = r.CallProgram(ctx, &CallInfo{
		Program:      token,
		Actor:        owner,
		State:        state,
		FunctionName: "mint",
		Params:       paramBytes,
		Fuel:         aLottaFuel,
	})
	require.NoError(err)

	// approve token transfers
	paramBytes, err = borsh.Serialize(struct {
		Spender codec.Address
		Amount  uint64
	}{
		Spender: ammAddress,
		Amount:  amount,
	})

	_, err = r.CallProgram(ctx, &CallInfo{
		Program:      token,
		Actor:        actor,
		State:        state,
		FunctionName: "approve",
		Params:       paramBytes,
		Fuel:         aLottaFuel,
	})
	require.NoError(err)

	paramBytes, err = borsh.Serialize(struct {
		Owner   codec.Address
		Spender codec.Address
	}{
		Owner:   actor,
		Spender: ammAddress,
	})
	require.NoError(err)

	result, err := r.CallProgram(ctx, &CallInfo{
		Program:      token,
		Actor:        actor,
		State:        state,
		FunctionName: "allowance",
		Params:       paramBytes,
		Fuel:         aLottaFuel,
	})
	require.NoError(err)
	var amountOfAllowance uint64

	err = borsh.Deserialize(&amountOfAllowance, result)
	require.Equal(uint64(amount), amountOfAllowance)
}
