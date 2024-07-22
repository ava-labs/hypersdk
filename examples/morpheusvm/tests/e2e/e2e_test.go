// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package e2e_test

import (
	"testing"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/tests/e2e"

	lrpc "github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
	ginkgo "github.com/onsi/ginkgo/v2"
)

var (
	_       e2e.Backend = (*morpheusVM)(nil)
	factory *auth.ED25519Factory
	addr    codec.Address
)

func init() {
	// Load default pk
	privBytes, err := codec.LoadHex(
		"323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7", //nolint:lll
		ed25519.PrivateKeyLen,
	)
	if err != nil {
		panic(err)
	}
	priv := ed25519.PrivateKey(privBytes)
	factory = auth.NewED25519Factory(priv)
	addr = auth.NewED25519Address(priv.PublicKey())
}

func TestE2e(t *testing.T) {
	ginkgo.RunSpecs(t, "morpheusvm e2e test suites")
}

type morpheusVM struct{}

func (morpheusVM) ID() ids.ID {
	return consts.ID
}

func (morpheusVM) Name() string {
	return consts.Name
}

func (morpheusVM) HRP() string {
	return consts.HRP
}

func (morpheusVM) AuthFactory() chain.AuthFactory {
	return factory
}

func (morpheusVM) NextAction() chain.Action {
	return &actions.Transfer{
		To:    addr,
		Value: 1,
	}
}

func (morpheusVM) NewJSONRPCClient(uri string, networkID uint32, chainID ids.ID) e2e.CustomClient {
	return lrpc.NewJSONRPCClient(uri, networkID, chainID)
}

var _ = ginkgo.BeforeSuite(func() {
	e2e.SetBackend(morpheusVM{})
})
