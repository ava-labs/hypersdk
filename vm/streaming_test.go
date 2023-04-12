// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/crypto"
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

func GenerateMockTx(require *require.Assertions, ctrl *gomock.Controller, action chain.Action, vm *VM) *chain.Transaction {
	tx := chain.NewTx(
		&chain.Base{
			UnitPrice: uint64(100),
			Timestamp: time.Now().Unix() + 10,
			ChainID:   ids.GenerateTestID(),
		},
		nil,
		action,
	)

	authFactory := chain.NewMockAuthFactory(ctrl)
	auth := chain.NewMockAuth(ctrl)
	priv, err := crypto.GeneratePrivateKey()
	require.NoError(err)
	pk := priv.PublicKey()
	auth.EXPECT().Payer().AnyTimes().Return(pk[:])
	auth.EXPECT().Marshal(gomock.Any()).Times(2)
	authFactory.EXPECT().Sign(gomock.Any(), gomock.Any()).Times(1).Return(auth, nil)
	tx_signed, err := tx.Sign(authFactory, vm.actionRegistry, vm.authRegistry)
	return tx_signed
}

func TestStreaming(t *testing.T) {
	// require := require.New(t)
	// // c := make(chan bool)
	// // Create a VM instance
	// ctrl := gomock.NewController(t)
	// defer ctrl.Finish()
	// tracer, _ := trace.New(&trace.Config{Enabled: false})
	// controller := NewMockController(ctrl)

	// // Action and auth registeries
	// actionParser := codec.NewTypeParser[chain.Action, *warp.Message]()
	// actionParser.Register(&chain.MockAction{}, func(*codec.Packer, *warp.Message) (chain.Action, error) {
	// 	return chain.NewMockAction(ctrl), nil
	// }, false)
	// actionRegistry := chain.ActionRegistry(actionParser)
	// authParser := codec.NewTypeParser[chain.Auth, *warp.Message]()
	// authParser.Register(&chain.MockAuth{}, func(*codec.Packer, *warp.Message) (chain.Auth, error) {
	// 	auth := chain.NewMockAuth(ctrl)
	// 	auth.EXPECT().AsyncVerify(gomock.Any()).Times(1).Return(nil)
	// 	return auth, nil
	// }, false)
	// authRegistry := chain.AuthRegistry(authParser)
	// // create a block
	// // create a block with "Unknown" status
	// blk := &chain.StatelessBlock{
	// 	StatefulBlock: &chain.StatefulBlock{
	// 		Prnt:      ids.GenerateTestID(),
	// 		Hght:      10000,
	// 		UnitPrice: 1000,
	// 		BlockCost: 100,
	// 	},
	// }
	// // blkID := blk.ID()
	// vm := &VM{
	// 	snowCtx: &snow.Context{Log: logging.NoLog{}, Metrics: ametrics.NewOptionalGatherer()},
	// 	tracer:  tracer,

	// 	blocks:         &cache.LRU[ids.ID, *chain.StatelessBlock]{Size: 3},
	// 	verifiedBlocks: make(map[ids.ID]*chain.StatelessBlock),
	// 	seen:           emap.NewEMap[*chain.Transaction](),
	// 	mempool:        mempool.New[*chain.Transaction](tracer, 100, 32, nil),
	// 	acceptedQueue:  make(chan *chain.StatelessBlock, 1024), // don't block on queue
	// 	c:              controller,
	// 	actionRegistry: actionRegistry,
	// 	authRegistry:   authRegistry,
	// }
	// vm.ready = make(chan struct{})
	// vm.stop = make(chan struct{})
	// vm.seenValidityWindow = make(chan struct{})

	// // Init metrics (called in [Accepted])
	// gatherer := ametrics.NewMultiGatherer()
	// m, err := newMetrics(gatherer)
	// if err != nil {
	// 	t.Fatal(err)
	// }
	// vm.metrics = m
	// require.NoError(vm.snowCtx.Metrics.Register(gatherer))
	// // set the vm ready
	// action := chain.NewMockAction(ctrl)
	// action.EXPECT().Marshal(gomock.Any()).Times(2)
	// // Create tx to be issued
	// tx := GenerateMockTx(require, ctrl, action, vm)
	// fmt.Println("TX Signed!", tx.Bytes())

	// // Create the decision server
	// addr := "localhost:8080"
	// u := url.URL{Scheme: "ws", Host: addr}
	// vm.decisionsServer = pubsub.New(addr, vm.decisionServerCallback, logging.NoLog{}, pubsub.NewDefaultServerConfig())
	// // start the server
	// go vm.decisionsServer.Start()
	// // create a client connection
	// <-time.After(100 * time.Millisecond)
	// client_con, err := NewDecisionRPCClient(u.String())
	// require.NoError(err)
	// // // issues tx to the VM
	// // put the block into the cache "vm.blocks"
	// // and delete from "vm.verifiedBlocks"
	// ctx := context.TODO()
	// rules := chain.NewMockRules(ctrl)
	// rules.EXPECT().GetValidityWindow().Return(int64(60))
	// controller.EXPECT().Rules(gomock.Any()).Return(rules)

	// vm.Accepted(ctx, blk)

	// client_con.IssueTx(tx)
	// // tell the vm it is ready
	// vm.ready <- struct{}{}
	// <-time.After(time.Second)

	// <-c
	// the decisionServer submits the tx to the VM
	// the client listens for a response from the VM
}
