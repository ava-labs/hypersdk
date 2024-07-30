// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"net/http"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/lineagevm/consts"
	"github.com/ava-labs/hypersdk/examples/lineagevm/genesis"
	"github.com/ava-labs/hypersdk/fees"
)

type JSONRPCServer struct {
	c Controller
}

func NewJSONRPCServer(c Controller) *JSONRPCServer {
	return &JSONRPCServer{c}
}

type GenesisReply struct {
	Genesis *genesis.Genesis `json:"genesis"`
}

func (j *JSONRPCServer) Genesis(_ *http.Request, _ *struct{}, reply *GenesisReply) (err error) {
	reply.Genesis = j.c.Genesis()
	return nil
}

type TxArgs struct {
	TxID ids.ID `json:"txId"`
}

type TxReply struct {
	Timestamp int64           `json:"timestamp"`
	Success   bool            `json:"success"`
	Units     fees.Dimensions `json:"units"`
	Fee       uint64          `json:"fee"`
}

func (j *JSONRPCServer) Tx(req *http.Request, args *TxArgs, reply *TxReply) error {
	ctx, span := j.c.Tracer().Start(req.Context(), "Server.Tx")
	defer span.End()

	found, t, success, units, fee, err := j.c.GetTransaction(ctx, args.TxID)
	if err != nil {
		return err
	}
	if !found {
		return ErrTxNotFound
	}
	reply.Timestamp = t
	reply.Success = success
	reply.Units = units
	reply.Fee = fee
	return nil
}

type BalanceArgs struct {
	Address string `json:"address"`
}

type BalanceReply struct {
	Amount uint64 `json:"amount"`
}

func (j *JSONRPCServer) Balance(req *http.Request, args *BalanceArgs, reply *BalanceReply) error {
	ctx, span := j.c.Tracer().Start(req.Context(), "Server.Balance")
	defer span.End()

	addr, err := codec.ParseAddressBech32(consts.HRP, args.Address)
	if err != nil {
		return err
	}
	balance, err := j.c.GetBalanceFromState(ctx, addr)
	if err != nil {
		return err
	}
	reply.Amount = balance
	return err
}

type GetProfessorDetailsArgs struct {
	ProfessorID string `json:"address"`
}

type GetProfessorDetailsReply struct {
	Name       string          `json:"name"`
	Year       uint16          `json:"year"`
	University string          `json:"university"`
	Students   []codec.Address `json:"students"`
}

func (j *JSONRPCServer) GetProfessorDetails(req *http.Request, args *GetProfessorDetailsArgs, reply *GetProfessorDetailsReply) error {
	ctx, span := j.c.Tracer().Start(req.Context(), "Server.GetProfessorDetails")
	defer span.End()

	addr, err := codec.ParseAddressBech32(consts.HRP, args.ProfessorID)
	if err != nil {
		return err
	}
	_, name, year, university, students, err := j.c.GetProfessorFromState(ctx, addr)
	if err != nil {
		return err
	}

	reply.Name = name
	reply.Year = year
	reply.University = university
	reply.Students = students
	return err
}

type LineageCheckArgs struct {
	ProfessorName string `json:"professorName"`
	StudentName   string `json:"studentName"`
}

type LineageCheckReply struct {
	ExistsPath bool `json:"existsPath"`
}

func (j *JSONRPCServer) LineageCheck(req *http.Request, args *LineageCheckArgs, reply *LineageCheckReply) error {
	ctx, span := j.c.Tracer().Start(req.Context(), "Server.DoesLineageExist")
	defer span.End()

	existsPath, err := j.c.DoesLineageExist(ctx, args.ProfessorName, args.StudentName)
	if err != nil {
		return err
	}

	reply.ExistsPath = existsPath
	return err
}
