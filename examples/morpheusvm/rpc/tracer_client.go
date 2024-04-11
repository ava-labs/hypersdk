// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"context"
)

func (cli *JSONRPCClient) TraceAction(ctx context.Context, req TraceTxArgs) (*TraceTxReply, error) {
	resp := new(TraceTxReply)
	err := cli.requester.SendRequest(
		ctx,
		"traceTx",
		&req,
		resp,
	)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
