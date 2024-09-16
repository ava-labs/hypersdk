// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"context"
	"log"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
)

func main() {
	client := jsonrpc.NewJSONRPCClient("http://127.0.0.1:9650/ext/bc/" + consts.Name)

	addr, err := codec.ParseAddressBech32(consts.HRP, "morpheus1qrzvk4zlwj9zsacqgtufx7zvapd3quufqpxk5rsdd4633m4wz2fdjk97rwu")
	if err != nil {
		log.Fatal(err)
	}

	transfer := &actions.Transfer{
		To:    addr,
		Value: 1,
		Memo:  []byte("test"),
	}
	transferResults, errorString, err := client.ExecuteAction(context.Background(), transfer, 0, addr)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("transferResults: %v, errorString: %v", transferResults, errorString)
}
