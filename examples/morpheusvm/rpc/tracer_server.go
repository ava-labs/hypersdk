// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"net/http"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/shim"
	"github.com/ava-labs/hypersdk/state"
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
	UsedGas     uint64 `json:"usedGas"`
	Output      []byte `json:"output"`
	WarpMessage bool   `json:"warpMessage"` // TODO: output full details
	StateKeys   []byte `json:"stateKeys"`
	Error       string `json:"error"`
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
	traced := shim.NewStateKeyTracer(mu)
	args.Action.SetLogger(j.c.Logger())
	success, actionCUs, output, warpMessage, err := args.Action.Execute(
		ctx, r, traced, t, args.Actor, args.TxID, args.WarpVerified,
	)
	if err != nil {
		return err
	}

	reply.Timestamp = t
	reply.Success = success
	reply.CUs = actionCUs
	reply.Output = output
	reply.WarpMessage = warpMessage != nil
	reply.Error = args.Action.ExecutionError()
	reply.UsedGas = args.Action.UsedGas()
	p := codec.NewWriter(0, consts.MaxInt)
	actions.MarshalKeys(traced.Keys, p)
	if err := p.Err(); err != nil {
		return err
	}
	reply.StateKeys = p.Bytes()
	return nil
}
