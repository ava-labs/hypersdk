// (c) 2019-2020, Ava Labs, Inc.
//
// This file is a derived work, based on the go-ethereum library whose original
// notices appear below.
//
// It is distributed under a license compatible with the licensing terms of the
// original code from which it is derived.
//
// Much love to the original authors for their work.
// **********
// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package actions

import (
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ethereum/go-ethereum/common"
)

type Log struct {
	// Consensus fields:
	// address of the contract that generated the event
	Address common.Address `json:"address" serialize:"true"`
	// list of topics provided by the contract.
	Topics []common.Hash `json:"topics" serialize:"true"`
	// supplied by the contract, usually ABI-encoded
	Data []byte `json:"data" serialize:"true"`

	// Derived fields. These fields are filled in by the node
	// but not secured by consensus.
	// block in which the transaction was included
	BlockNumber uint64 `json:"blockNumber" serialize:"true"`
	// hash of the transaction
	TxHash common.Hash `json:"transactionHash" serialize:"true"`
	// index of the transaction in the block
	TxIndex uint64 `json:"transactionIndex" serialize:"true"`
	// hash of the block in which the transaction was included
	BlockHash common.Hash `json:"blockHash" serialize:"true"`
	// index of the log in the block
	Index uint64 `json:"logIndex" serialize:"true"`

	// The Removed field is true if this log was reverted due to a chain reorganisation.
	// You must pay attention to this field if you receive logs through a filter query.
	Removed bool `json:"removed" serialize:"true"`
}

// convertLogs converts an array of logs (types.Log) to an array of Logs
// (HyperEVM)
//
// using codec.LinearCodec on (types.Log) fails due to the struct fields not
// having the serialize tag and so we convert (types.Log) to our version of Log
// which has the serialize tag
func convertLogs(logs []*types.Log) []Log {
	result := make([]Log, len(logs))
	for i, log := range logs {
		result[i] = Log{
			Address:     log.Address,
			Topics:      log.Topics,
			Data:        log.Data,
			BlockNumber: log.BlockNumber,
			TxHash:      log.TxHash,
			TxIndex:     uint64(log.TxIndex),
			BlockHash:   log.BlockHash,
			Index:       uint64(log.Index),
			Removed:     log.Removed,
		}
	}
	return result
}
