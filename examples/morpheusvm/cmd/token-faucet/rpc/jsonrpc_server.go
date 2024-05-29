// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"net/http"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/tokenvm/consts"
)

type JSONRPCServer struct {
	m Manager
}

func NewJSONRPCServer(m Manager) *JSONRPCServer {
	return &JSONRPCServer{m}
}

type FaucetAddressReply struct {
	Address string `json:"address"`
}

func (j *JSONRPCServer) FaucetAddress(req *http.Request, _ *struct{}, reply *FaucetAddressReply) (err error) {
	addr, err := j.m.GetFaucetAddress(req.Context())
	if err != nil {
		return err
	}
	reply.Address = codec.MustAddressBech32(consts.HRP, addr)
	return nil
}

type ChallengeReply struct {
	Salt       []byte `json:"salt"`
	Difficulty uint16 `json:"difficulty"`
}

func (j *JSONRPCServer) Challenge(req *http.Request, _ *struct{}, reply *ChallengeReply) (err error) {
	salt, difficulty, err := j.m.GetChallenge(req.Context())
	if err != nil {
		return err
	}
	reply.Salt = salt
	reply.Difficulty = difficulty
	return nil
}

type SolveChallengeArgs struct {
	Address  string `json:"address"`
	Salt     []byte `json:"salt"`
	Solution []byte `json:"solution"`
}

type SolveChallengeReply struct {
	TxID   ids.ID `json:"txID"`
	Amount uint64 `json:"amount"`
}

func (j *JSONRPCServer) SolveChallenge(req *http.Request, args *SolveChallengeArgs, reply *SolveChallengeReply) error {
	addr, err := codec.ParseAddressBech32(consts.HRP, args.Address)
	if err != nil {
		return err
	}
	txID, amount, err := j.m.SolveChallenge(req.Context(), addr, args.Salt, args.Solution)
	if err != nil {
		return err
	}
	reply.TxID = txID
	reply.Amount = amount
	return nil
}
