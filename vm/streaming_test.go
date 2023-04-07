// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	ametrics "github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/cache"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/emap"
	"github.com/ava-labs/hypersdk/mempool"
	"github.com/ava-labs/hypersdk/pubsub"
	trace "github.com/ava-labs/hypersdk/trace"
	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func CreateFakeTX(require *require.Assertions, ctrl *gomock.Controller) {

	// actionRegistry := codec.NewTypeParser[chain.Action, *warp.Message, bool]
	// require.NoError(
	// 	actionRegistry.Register(&chain.MockAction{}, func(*codec.Packer, chain.Action) error {
	// 		return nil
	// 	}),
	// )
	// authRegistry := codec.NewTypeParser[chain.Auth]()
	// require.NoError(
	// 	authRegistry.Register(&chain.MockAuth{}, func(p *codec.Packer) (chain.Auth, error) {
	// 		return chain.NewMockAuth(ctrl), nil
	// 	}),
	// )
	// action := chain.NewMockAction(ctrl)
	// action.EXPECT().Marshal(gomock.Any()).Times(3)
	// priv, err := crypto.GeneratePrivateKey()
	// require.NoError(err)
	// pk := priv.PublicKey()
	// auth := chain.NewMockAuth(ctrl)
	// auth.EXPECT().Payer().AnyTimes().Return(pk[:])
	// auth.EXPECT().Marshal(gomock.Any()).Times(1)
	// auth.EXPECT().AsyncVerify(gomock.Any()).Times(1).Return(nil)
	// authFactory := chain.NewMockAuthFactory(ctrl)
	// authFactory.EXPECT().Sign(gomock.Any(), gomock.Any()).Times(1).Return(auth, nil)
	// tx := chain.NewTx(
	// 	&chain.Base{
	// 		UnitPrice: uint64(100),
	// 		Timestamp: time.Now().Unix() + 10,
	// 	},
	// 	nil,
	// 	action,
	// )
	// require.NoError(tx.Sign(authFactory))
	// sigVerify, err := tx.Init(ctx, actionRegistry, authRegistry)
	// require.NoError(err)
	// require.NoError(sigVerify())
	// txm.Add(ctx, []*chain.Transaction{tx})
}

func TestStreaming(t *testing.T) {
	require := require.New(t)
	// Create a VM instance
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	tracer, _ := trace.New(&trace.Config{Enabled: false})
	controller := NewMockController(ctrl)
	// create a block with "Unknown" status
	// blk := &chain.StatelessBlock{
	// 	StatefulBlock: &chain.StatefulBlock{
	// 		Prnt:      ids.GenerateTestID(),
	// 		Hght:      10000,
	// 		UnitPrice: 1000,
	// 		BlockCost: 100,
	// 	},
	// }
	// blkID := blk.ID()

	vm := VM{
		snowCtx: &snow.Context{Log: logging.NoLog{}, Metrics: ametrics.NewOptionalGatherer()},
		tracer:  tracer,

		blocks:         &cache.LRU[ids.ID, *chain.StatelessBlock]{Size: 3},
		verifiedBlocks: make(map[ids.ID]*chain.StatelessBlock),
		seen:           emap.NewEMap[*chain.Transaction](),
		mempool:        mempool.New[*chain.Transaction](tracer, 100, 32, nil),
		acceptedQueue:  make(chan *chain.StatelessBlock, 1024), // don't block on queue
		c:              controller,
	}
	// Init metrics (called in [Accepted])
	gatherer := ametrics.NewMultiGatherer()
	m, err := newMetrics(gatherer)
	if err != nil {
		t.Fatal(err)
	}
	vm.metrics = m
	require.NoError(vm.snowCtx.Metrics.Register(gatherer))
	// Create the decision server
	addr := "localhost:8080"
	u := url.URL{Scheme: "ws", Host: addr}
	vm.decisionsServer = pubsub.New(addr, vm.decisionServerCallback, logging.NoLog{}, pubsub.NewDefaultServerConfig())

	// Create tx to be issued
	action := chain.NewMockAction(ctrl)
	action.EXPECT().Marshal(gomock.Any()).Times(2)
	tx := chain.NewTx(
		&chain.Base{
			UnitPrice: uint64(100),
			Timestamp: time.Now().Unix() + 10,
			ChainID:   ids.GenerateTestID(),
		},
		nil,
		action,
	)
	actionParser := codec.NewTypeParser[chain.Action, *warp.Message]()
	actionParser.Register(&chain.MockAction{}, func(*codec.Packer, *warp.Message) (chain.Action, error) {
		return chain.NewMockAction(ctrl), nil
	}, false)
	actionRegistry := chain.ActionRegistry(actionParser)
	authParser := codec.NewTypeParser[chain.Auth, *warp.Message]()
	authParser.Register(&chain.MockAuth{}, func(*codec.Packer, *warp.Message) (chain.Auth, error) {
		return chain.NewMockAuth(ctrl), nil
	}, false)
	authRegistry := chain.AuthRegistry(authParser)

	authFactory := chain.NewMockAuthFactory(ctrl)
	auth := chain.NewMockAuth(ctrl)
	priv, err := crypto.GeneratePrivateKey()
	require.NoError(err)
	pk := priv.PublicKey()
	auth.EXPECT().Payer().AnyTimes().Return(pk[:])
	auth.EXPECT().Marshal(gomock.Any()).Times(1)
	authFactory.EXPECT().Sign(gomock.Any(), gomock.Any()).Times(1).Return(auth, nil)
	tx_signed, err := tx.Sign(authFactory, actionRegistry, authRegistry)
	fmt.Println("TX Signed!", tx_signed.Bytes())
	// start the server
	go vm.decisionsServer.Start()
	// create a client connection
	<-time.After(10 * time.Millisecond)
	client_con, err := NewDecisionRPCClient(u.String())
	require.NoError(err)
	// issues tx to the VM
	client_con.IssueTx(tx)
	// the decisionServer submits the tx to the VM
	// the client listens for a response from the VM

}
