package main

import (
	"context"
	"fmt"
	"log"

	lrpc "github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
	"github.com/ava-labs/hypersdk/rpc"
)

func main() {
	// privBytes, err := codec.LoadHex(
	// 	"323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7", //nolint:lll
	// 	ed25519.PrivateKeyLen,
	// )
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// priv := ed25519.PrivateKey(privBytes)
	// factory := auth.NewED25519Factory(priv)
	// rsender := auth.NewED25519Address(priv.PublicKey())
	// sender := codec.MustAddressBech32(consts.HRP, rsender)

	url := "http://localhost:9650/ext/bc/morpheusvm"
	cli := rpc.NewJSONRPCClient(url)

	networkId, subnetId, chainId, err := cli.Network(context.TODO())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(networkId, subnetId, chainId)

	lcli := lrpc.NewJSONRPCClient(url, networkId, chainId)
	balance, err := lcli.Balance(context.TODO(), "morpheus1qrzvk4zlwj9zsacqgtufx7zvapd3quufqpxk5rsdd4633m4wz2fdjk97rwu")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(balance)

}
