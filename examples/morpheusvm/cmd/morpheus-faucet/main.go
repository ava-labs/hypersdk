package main

import (
	"github.com/ava-labs/hypersdk/chain"

	"context"
	"fmt"
	"log"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	lconsts "github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	lrpc "github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
	"github.com/ava-labs/hypersdk/rpc"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/utils"
)

const amtStr = "10.00"

func transferCoins(to string) error {
	toAddr, err := codec.ParseAddressBech32(lconsts.HRP, to)
	if err != nil {
		return fmt.Errorf("failed to parse to address: %w", err)
	}

	amt, err := utils.ParseBalance(amtStr, consts.Decimals)
	if err != nil {
		return fmt.Errorf("failed to parse amount: %w", err)
	}

	privBytes, err := codec.LoadHex(
		"323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7", //nolint:lll
		ed25519.PrivateKeyLen,
	)
	if err != nil {
		return fmt.Errorf("failed to load private key: %w", err)
	}

	priv := ed25519.PrivateKey(privBytes)
	factory := auth.NewED25519Factory(priv)

	url := "http://localhost:9650/ext/bc/morpheusvm"
	cli := rpc.NewJSONRPCClient(url)

	networkId, subnetId, chainId, err := cli.Network(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to get network info: %w", err)
	}
	fmt.Println(networkId, subnetId, chainId)

	lcli := lrpc.NewJSONRPCClient(url, networkId, chainId)
	balanceBefore, err := lcli.Balance(context.TODO(), to)
	if err != nil {
		return fmt.Errorf("failed to get balance: %w", err)
	}
	fmt.Printf("Balance before: %s\n", utils.FormatBalance(balanceBefore, consts.Decimals))

	parser, err := lcli.Parser(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to get parser: %w", err)
	}

	submit, _, _, err := cli.GenerateTransaction(
		context.TODO(),
		parser,
		[]chain.Action{&actions.Transfer{
			To:    toAddr,
			Value: amt,
		}},
		factory,
	)
	if err != nil {
		return fmt.Errorf("failed to generate transaction: %w", err)
	}

	err = submit(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to submit transaction: %w", err)
	}

	balanceAfter, err := lcli.Balance(context.TODO(), to)
	if err != nil {
		return fmt.Errorf("failed to get balance: %w", err)
	}
	fmt.Printf("Balance after: %s\n", utils.FormatBalance(balanceAfter, consts.Decimals))

	return nil
}

func main() {

	err := transferCoins("morpheus1qrugaulrfx7hm6vnxujyewzu0rmqugqztg69rkzdrgv0afzyfcfkcxtyw96")
	if err != nil {
		log.Fatal(err)
	}
}
