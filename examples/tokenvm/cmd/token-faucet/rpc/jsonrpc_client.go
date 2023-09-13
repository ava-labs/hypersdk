// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"context"
	"strings"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/requester"
)

const (
	JSONRPCEndpoint = "/faucet"
)

type JSONRPCClient struct {
	requester *requester.EndpointRequester
}

// New creates a new client object.
func NewJSONRPCClient(uri string) *JSONRPCClient {
	uri = strings.TrimSuffix(uri, "/")
	uri += JSONRPCEndpoint
	req := requester.New(uri, "faucet")
	return &JSONRPCClient{
		requester: req,
	}
}

func (cli *JSONRPCClient) FaucetAddress(ctx context.Context) (string, error) {
	resp := new(FaucetAddressReply)
	err := cli.requester.SendRequest(
		ctx,
		"faucetAddress",
		nil,
		resp,
	)
	return resp.Address, err
}

func (cli *JSONRPCClient) Challenge(ctx context.Context) ([]byte, uint16, error) {
	resp := new(ChallengeReply)
	err := cli.requester.SendRequest(
		ctx,
		"challenge",
		nil,
		resp,
	)
	return resp.Salt, resp.Difficulty, err
}

func (cli *JSONRPCClient) SolveChallenge(ctx context.Context, addr string, salt []byte, solution []byte) (ids.ID, uint64, error) {
	resp := new(SolveChallengeReply)
	err := cli.requester.SendRequest(
		ctx,
		"solveChallenge",
		&SolveChallengeArgs{
			Address:  addr,
			Salt:     salt,
			Solution: solution,
		},
		resp,
	)
	return resp.TxID, resp.Amount, err
}
