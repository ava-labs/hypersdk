// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"net/http"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/tokenvm/cmd/token-feed/manager"
	"github.com/ava-labs/hypersdk/examples/tokenvm/consts"
)

type JSONRPCServer struct {
	m Manager
}

func NewJSONRPCServer(m Manager) *JSONRPCServer {
	return &JSONRPCServer{m}
}

type FeedInfoReply struct {
	Address string `json:"address"`
	Fee     uint64 `json:"fee"`
}

func (j *JSONRPCServer) FeedInfo(req *http.Request, _ *struct{}, reply *FeedInfoReply) (err error) {
	addr, fee, err := j.m.GetFeedInfo(req.Context())
	if err != nil {
		return err
	}
	reply.Address = codec.MustAddressBech32(consts.HRP, addr)
	reply.Fee = fee
	return nil
}

type FeedReply struct {
	Feed []*manager.FeedObject `json:"feed"`
}

func (j *JSONRPCServer) Feed(req *http.Request, _ *struct{}, reply *FeedReply) (err error) {
	feed, err := j.m.GetFeed(req.Context())
	if err != nil {
		return err
	}
	reply.Feed = feed
	return nil
}
