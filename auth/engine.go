// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package auth

import "github.com/ava-labs/hypersdk/chain"

type Engine interface {
	GetBatchVerifier(cores int, count int) chain.AuthBatchVerifier
	Cache(auth chain.Auth)
}

type Engines map[uint8]Engine

func (e Engines) GetAuthBatchVerifier(authTypeID uint8, cores int, count int) (chain.AuthBatchVerifier, bool) {
	engine, ok := e[authTypeID]
	if !ok {
		return nil, false
	}
	return engine.GetBatchVerifier(cores, count), true
}
