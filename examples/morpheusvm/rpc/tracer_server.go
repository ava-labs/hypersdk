// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"net/http"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ethereum/go-ethereum/rlp"
)

type TraceTxArgs struct {
	Action       actions.EvmCall `json:"action"`
	TxID         ids.ID          `json:"txId"`
	Actor        codec.Address   `json:"actor"`
	WarpVerified bool            `json:"verifiedWarp"`
}

type TraceTxReply struct {
	Timestamp   int64  `json:"timestamp"`
	Success     bool   `json:"success"`
	CUs         uint64 `json:"units"`
	Output      []byte `json:"output"`
	WarpMessage bool   `json:"warpMessage"` // TODO: output full details
	AccessList  []byte `json:"stateKeys"`
}

func (j *JSONRPCServer) TraceTx(
	req *http.Request, args *TraceTxArgs, reply *TraceTxReply,
) error {
	ctx, span := j.c.Tracer().Start(req.Context(), "Server.TraceTx")
	defer span.End()

	t := time.Now().UnixMilli()
	r := j.c.Rules(t)

	view, err := j.c.View()
	if err != nil {
		return err
	}

	mu := state.NewSimpleMutable(view)
	args.Action.SetLogger(j.c.Logger())
	action := args.Action.TraceAction()
	success, actionCUs, output, warpMessage, err := action.Execute(
		ctx, r, mu, t, args.Actor, args.TxID, args.WarpVerified,
	)
	if err != nil {
		return err
	}

	reply.Timestamp = t
	reply.Success = success
	reply.CUs = actionCUs
	reply.Output = output
	reply.WarpMessage = warpMessage != nil
	reply.AccessList, err = rlp.EncodeToBytes(args.Action.AccessList)
	return err
}
