// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package auth

import (
	"context"
	"errors"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
)

var (
	_         chain.ExecutableAuth      = (*OneOf)(nil)
	_         chain.Auth[*OneOf]        = (*OneOf)(nil)
	_         chain.AuthFactory[*OneOf] = (*OneOfFactory)(nil)
	ErrNoAuth                           = errors.New("no auth populated")
)

type OneOf struct {
	BLS       *BLS       `canoto:"pointer,1,auth"`
	ED25519   *ED25519   `canoto:"pointer,2,auth"`
	SECP256R1 *SECP256R1 `canoto:"pointer,3,auth"`

	canotoData canotoData_OneOf
}

func (a *OneOf) selectAuth() (chain.ExecutableAuth, bool) {
	switch {
	case a.BLS != nil:
		return a.BLS, true
	case a.ED25519 != nil:
		return a.ED25519, true
	case a.SECP256R1 != nil:
		return a.SECP256R1, true
	default:
		return nil, false
	}
}

func (a *OneOf) ComputeUnits() uint64 {
	auth, ok := a.selectAuth()
	if !ok {
		return 0
	}
	return auth.ComputeUnits()
}

func (a *OneOf) Verify(ctx context.Context, msg []byte) error {
	auth, ok := a.selectAuth()
	if !ok {
		return ErrNoAuth
	}
	return auth.Verify(ctx, msg)
}

func (a *OneOf) Actor() codec.Address {
	auth, ok := a.selectAuth()
	if !ok {
		return codec.EmptyAddress
	}
	return auth.Actor()
}

func (a *OneOf) Sponsor() codec.Address {
	auth, ok := a.selectAuth()
	if !ok {
		return codec.EmptyAddress
	}
	return auth.Sponsor()
}

type OneOfFactory struct {
	ed25519Factory   *ED25519Factory   `canoto:"pointer,1,factory"`
	blsFactory       *BLSFactory       `canoto:"pointer,2,factory"`
	secp256R1Factory *SECP256R1Factory `canoto:"pointer,3,factory"`

	canotoData canotoData_OneOfFactory
}

func NewOneOfFactory(bytes []byte) (*OneOfFactory, error) {
	factory := &OneOfFactory{}
	if err := factory.UnmarshalCanoto(bytes); err != nil {
		return nil, err
	}
	return factory, nil
}

func (f *OneOfFactory) Sign(msg []byte) (*OneOf, error) {
	switch {
	case f.ed25519Factory != nil:
		ed25519, err := f.ed25519Factory.Sign(msg)
		if err != nil {
			return nil, err
		}
		return &OneOf{ED25519: ed25519}, nil
	case f.blsFactory != nil:
		bls, err := f.blsFactory.Sign(msg)
		if err != nil {
			return nil, err
		}
		return &OneOf{BLS: bls}, nil
	case f.secp256R1Factory != nil:
		secp256R1, err := f.secp256R1Factory.Sign(msg)
		if err != nil {
			return nil, err
		}
		return &OneOf{SECP256R1: secp256R1}, nil
	default:
		return nil, ErrNoAuth
	}
}

func (f *OneOfFactory) MaxUnits() (uint64, uint64) {
	switch {
	case f.ed25519Factory != nil:
		return f.ed25519Factory.MaxUnits()
	case f.blsFactory != nil:
		return f.blsFactory.MaxUnits()
	case f.secp256R1Factory != nil:
		return f.secp256R1Factory.MaxUnits()
	default:
		return 0, 0
	}
}

func (f *OneOfFactory) Address() codec.Address {
	switch {
	case f.ed25519Factory != nil:
		return f.ed25519Factory.Address()
	case f.blsFactory != nil:
		return f.blsFactory.Address()
	case f.secp256R1Factory != nil:
		return f.secp256R1Factory.Address()
	default:
		return codec.EmptyAddress
	}
}
