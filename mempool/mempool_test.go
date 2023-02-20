// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package mempool

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/trace"
)

func TestMempool(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.TODO()
	tracer, _ := trace.New(&trace.Config{Enabled: false})
	txm := New(tracer, 3, 16, nil)
	actionRegistry := codec.NewTypeParser[chain.Action]()
	require.NoError(
		actionRegistry.Register(&chain.MockAction{}, func(*codec.Packer) (chain.Action, error) {
			return chain.NewMockAction(ctrl), nil
		}),
	)
	authRegistry := codec.NewTypeParser[chain.Auth]()
	require.NoError(
		authRegistry.Register(&chain.MockAuth{}, func(p *codec.Packer) (chain.Auth, error) {
			return chain.NewMockAuth(ctrl), nil
		}),
	)
	for _, i := range []int{100, 200, 300, 400} {
		action := chain.NewMockAction(ctrl)
		action.EXPECT().Marshal(gomock.Any()).Times(3)
		priv, err := crypto.GeneratePrivateKey()
		require.NoError(err)
		pk := priv.PublicKey()
		auth := chain.NewMockAuth(ctrl)
		auth.EXPECT().Payer().AnyTimes().Return(pk[:])
		auth.EXPECT().Marshal(gomock.Any()).Times(1)
		auth.EXPECT().AsyncVerify(gomock.Any()).Times(1).Return(nil)
		authFactory := chain.NewMockAuthFactory(ctrl)
		authFactory.EXPECT().Sign(gomock.Any(), gomock.Any()).Times(1).Return(auth, nil)
		tx := chain.NewTx(
			&chain.Base{
				UnitPrice: uint64(i),
				Timestamp: time.Now().Unix() + 10,
			},
			action,
		)
		require.NoError(tx.Sign(authFactory))
		sigVerify, err := tx.Init(ctx, actionRegistry, authRegistry)
		require.NoError(err)
		require.NoError(sigVerify())
		txm.Add(ctx, []*chain.Transaction{tx})
	}
	require.Equal(uint64(400), txm.PeekMax(ctx).Base.UnitPrice)
	require.Equal(uint64(200), txm.PeekMin(ctx).Base.UnitPrice)
	require.Equal(3, txm.Len(ctx))
}
