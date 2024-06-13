package runtime

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/x/programs/test"
)

func TestTokenAMM(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	token1Account := codec.CreateAddress(1, ids.GenerateTestID())
	token2Account := codec.CreateAddress(2, ids.GenerateTestID())
	ammAccount := codec.CreateAddress(3, ids.GenerateTestID())

	r := &testRuntime{
		Context: ctx,
		Runtime: NewRuntime(
			NewConfig(),
			logging.NoLog{},
			test.MapLoader{Programs: map[codec.Address]string{token1Account: "erc20_token", token2Account: "erc20_token", ammAccount: "amm"}}),
		StateDB:    test.StateLoader{Mu: test.NewTestDB()},
		DefaultGas: 900000000,
	}

	token1 := testProgram{Runtime: r, Address: token1Account}
	token2 := testProgram{Runtime: r, Address: token2Account}
	amm := testProgram{Runtime: r, Address: ammAccount}

	token1Owner := codec.CreateAddress(1, ids.GenerateTestID())
	token2Owner := codec.CreateAddress(2, ids.GenerateTestID())
	ammOwner := codec.CreateAddress(3, ids.GenerateTestID())

	// init token 1
	_, err := token1.CallWithActor(
		token1Owner,
		"init",
		"Token 1", "TKN1")
	require.NoError(err)

	// init token 2
	_, err = token2.CallWithActor(
		token2Owner,
		"init",
		"Token 2", "TKN2")
	require.NoError(err)

	// init amm

	_, err = amm.CallWithActor(
		ammOwner,
		"init",
		token1Account, token2Account)
	require.NoError(err)

	user1 := codec.CreateAddress(8, ids.GenerateTestID())
	setupUser(require, token1, token1Owner, user1, ammAccount, 5)
	setupUser(require, token2, token2Owner, user1, ammAccount, 5)
	AddLiquitityToUser(require, user1, token1, token2, amm, 5)

	result, err := amm.Call("total_supply")
	require.NoError(err)
	require.Equal(uint64(10), test.Into[uint64](result))

	user2 := codec.CreateAddress(9, ids.GenerateTestID())
	setupUser(require, token1, token1Owner, user2, ammAccount, 6)
	setupUser(require, token2, token2Owner, user2, ammAccount, 6)
	AddLiquitityToUser(require, user2, token1, token2, amm, 6)

	result, err = amm.Call("total_supply")
	require.NoError(err)
	require.Equal(uint64(22), test.Into[uint64](result))

	RemoveLiquidityFromUser(require, user1, token1, token2, amm, 10)
	result, err = amm.Call("total_supply")
	require.NoError(err)
	require.Equal(uint64(12), test.Into[uint64](result))

	RemoveLiquidityFromUser(require, user2, token1, token2, amm, 12)
	result, err = amm.Call("total_supply")
	require.NoError(err)
	require.Equal(uint64(0), test.Into[uint64](result))
}

func RemoveLiquidityFromUser(require *require.Assertions, actor codec.Address, token1 testProgram, token2 testProgram, amm testProgram, amount uint64) {
	_, err := amm.CallWithActor(
		actor,
		"remove_liquidity",
		amount)
	require.NoError(err)

	result, err := amm.Call(
		"balance_of",
		actor)
	require.NoError(err)
	require.Equal(uint64(0), test.Into[uint64](result))

	// confirm balance change for user in token 1
	result, err = token1.Call(
		"balance_of",
		actor)
	require.NoError(err)
	require.Equal(amount/2, test.Into[uint64](result))

	// confirm balance change for user in token 2
	result, err = token2.Call(
		"balance_of",
		actor)
	require.NoError(err)
	require.Equal(amount/2, test.Into[uint64](result))
}

func AddLiquitityToUser(require *require.Assertions, actor codec.Address, token1 testProgram, token2 testProgram, amm testProgram, amount uint64) {
	_, err := amm.CallWithActor(
		actor,
		"add_liquidity",
		amount, amount)
	require.NoError(err)

	// confirm liquidity
	result, err := amm.Call(
		"balance_of",
		actor)
	require.NoError(err)
	require.Equal(amount*2, test.Into[uint64](result))

	// confirm balance change for user in token 1
	result, err = token1.Call(
		"balance_of",
		actor)
	require.NoError(err)
	require.Equal(uint64(0), test.Into[uint64](result))

	// confirm balance change for user in token 2
	result, err = token2.Call(
		"balance_of",
		actor)
	require.NoError(err)
	require.Equal(uint64(0), test.Into[uint64](result))
}

func setupUser(require *require.Assertions, token testProgram, owner codec.Address, actor codec.Address, ammAddress codec.Address, amount uint64) {
	_, err := token.CallWithActor(
		owner,
		"mint",
		actor, amount)
	require.NoError(err)

	_, err = token.CallWithActor(
		actor,
		"approve",
		ammAddress, amount)
	require.NoError(err)

	result, err := token.Call(
		"allowance",
		actor, ammAddress)
	require.NoError(err)
	require.Equal(amount, test.Into[uint64](result))
}
