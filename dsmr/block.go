// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
)

type Block struct {
	ID          ids.ID `serialize:"true"`
	ParentID    ids.ID `serialize:"true"`
	BlockHeight uint64 `serialize:"true"`
	Time        int64  `serialize:"true"`

	Chunks []*NoVerifyChunkCertificate `serialize:"true"`

	bytes []byte
}

type ExecutionBlock[VerificationContext any] struct {
	Block

	vm VM[VerificationContext]
}

func (e *ExecutionBlock[V]) Verify(ctx context.Context, verificationContext V) error {
	for _, chunkCertificate := range e.Chunks {
		if err := chunkCertificate.Verify(ctx, verificationContext); err != nil {
			return err
		}
	}

	return e.vm.Verify(ctx, e)
}

func (e *ExecutionBlock[V]) Accept(ctx context.Context) error {
	return e.vm.Accept(ctx, e)
}

func (e *ExecutionBlock[V]) Reject(ctx context.Context) error {
	return e.vm.Reject(ctx, e)
}
