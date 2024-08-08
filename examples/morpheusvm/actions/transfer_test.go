// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"encoding/hex"
	"math"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/tstate"

	consts "github.com/ava-labs/hypersdk/consts"
	mconsts "github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
)

func TestTransferAction(t *testing.T) {
	req := require.New(t)
	ts := tstate.New(1)
	emptyBalanceKey := storage.BalanceKey(codec.EmptyAddress)
	oneAddr, err := createAddressWithByte(1)
	req.NoError(err)

	tests := []chaintest.ActionTest{
		{
			Name:  "ZeroTransfer",
			Actor: codec.EmptyAddress,
			Action: &Transfer{
				To:    codec.EmptyAddress,
				Value: 0,
			},
			ExpectedErr: ErrOutputValueZero,
		},
		{
			Name:  "InvalidStateKey",
			Actor: codec.EmptyAddress,
			Action: &Transfer{
				To:    codec.EmptyAddress,
				Value: 1,
			},
			State:       ts.NewView(make(state.Keys), map[string][]byte{}),
			ExpectedErr: tstate.ErrInvalidKeyOrPermission,
		},
		{
			Name:  "NotEnoughBalance",
			Actor: codec.EmptyAddress,
			Action: &Transfer{
				To:    codec.EmptyAddress,
				Value: 1,
			},
			State: func() state.Mutable {
				keys := make(state.Keys)
				keys.Add(string(emptyBalanceKey), state.Read)
				tsv := ts.NewView(keys, map[string][]byte{})
				return tsv
			}(),
			ExpectedErr: storage.ErrInvalidBalance,
		},
		{
			Name:  "SelfTransfer",
			Actor: codec.EmptyAddress,
			Action: &Transfer{
				To:    codec.EmptyAddress,
				Value: 1,
			},
			State: func() state.Mutable {
				keys := make(state.Keys)
				store := chaintest.NewInMemoryStore()
				req.NoError(storage.SetBalance(context.Background(), store, codec.EmptyAddress, 1))
				keys.Add(string(emptyBalanceKey), state.All)
				return ts.NewView(keys, store.Storage)
			}(),
			Assertion: func(ctx context.Context, t *testing.T, store state.Mutable) {
				require := require.New(t)
				balance, err := storage.GetBalance(ctx, store, codec.EmptyAddress)
				require.NoError(err)
				require.Equal(balance, uint64(1))
			},
		},
		{
			Name:  "OverflowBalance",
			Actor: codec.EmptyAddress,
			Action: &Transfer{
				To:    codec.EmptyAddress,
				Value: math.MaxUint64,
			},
			State: func() state.Mutable {
				keys := make(state.Keys)
				store := chaintest.NewInMemoryStore()
				req.NoError(storage.SetBalance(context.Background(), store, codec.EmptyAddress, 1))
				keys.Add(string(emptyBalanceKey), state.All)
				return ts.NewView(keys, store.Storage)
			}(),
			ExpectedErr: storage.ErrInvalidBalance,
		},
		{
			Name:  "SimpleTransfer",
			Actor: codec.EmptyAddress,
			Action: &Transfer{
				To:    oneAddr,
				Value: 1,
			},
			State: func() state.Mutable {
				keys := make(state.Keys)
				store := chaintest.NewInMemoryStore()
				req.NoError(storage.SetBalance(context.Background(), store, codec.EmptyAddress, 1))
				keys.Add(string(emptyBalanceKey), state.All)
				keys.Add(string(storage.BalanceKey(oneAddr)), state.All)
				return ts.NewView(keys, store.Storage)
			}(),
			Assertion: func(ctx context.Context, t *testing.T, store state.Mutable) {
				require := require.New(t)
				receiverBalance, err := storage.GetBalance(ctx, store, oneAddr)
				require.NoError(err)
				require.Equal(receiverBalance, uint64(1))
				senderBalance, err := storage.GetBalance(ctx, store, codec.EmptyAddress)
				require.NoError(err)
				require.Equal(senderBalance, uint64(0))
			},
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}

func createAddressWithByte(b byte) (codec.Address, error) {
	addrSlice := make([]byte, codec.AddressLen)
	for i := range addrSlice {
		addrSlice[i] = b
	}
	return codec.ToAddress(addrSlice)
}

func TestTransferMarshalSpec(t *testing.T) {
	// These specification tests provide hexadecimal representations of serialized Transfer objects.
	// The hex strings are used to ensure byte-perfect consistency between Go and TypeScript implementations.
	// This helps verify that both implementations serialize Transfer objects identically.
	addr1, err := codec.ParseAddressBech32(mconsts.HRP, "morpheus1qqds2l0ryq5hc2ddps04384zz6rfeuvn3kyvn77hp4n5sv3ahuh6wgkt57y")
	require.NoError(t, err)

	addr2, err := codec.ParseAddressBech32(mconsts.HRP, "morpheus1q8rc050907hx39vfejpawjydmwe6uujw0njx9s6skzdpp3cm2he5s036p07")
	require.NoError(t, err)

	emptyAddrString := "morpheus1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqxez33a"
	require.Equal(t, emptyAddrString, codec.MustAddressBech32(mconsts.HRP, codec.EmptyAddress))

	tests := []struct {
		name     string
		transfer Transfer
		expected string
	}{
		{
			name: "Zero value",
			transfer: Transfer{
				To:    addr1,
				Value: 0,
				Memo:  []byte("test memo"),
			},
			expected: "001b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa700000000000000000000000974657374206d656d6f",
		},
		{
			name: "Max uint64 value",
			transfer: Transfer{
				To:    addr1,
				Value: math.MaxUint64,
				Memo:  []byte("another memo"),
			},
			expected: "001b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7ffffffffffffffff0000000c616e6f74686572206d656d6f",
		},
		{
			name: "Empty address",
			transfer: Transfer{
				To:    codec.EmptyAddress,
				Value: 123,
				Memo:  []byte("memo"),
			},
			expected: "000000000000000000000000000000000000000000000000000000000000000000000000000000007b000000046d656d6f",
		},
		{
			name: "Empty memo",
			transfer: Transfer{
				To:    addr2,
				Value: 456,
				Memo:  []byte{},
			},
			expected: "01c787d1e57fae689589cc83d7488ddbb3ae724e7ce462c350b09a10c71b55f34800000000000001c800000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := codec.NewWriter(0, consts.NetworkSizeLimit)
			tt.transfer.Marshal(p)
			require.Equal(t, tt.expected, hex.EncodeToString(p.Bytes()))
		})
	}
}

func TestSignleTransferTxSignAndMarshalSpec(t *testing.T) {
	chainIDStr := "2c7iUW3kCDwRA9ZFd5bjZZc8iDy68uAsFSBahjqSZGttiTDSNH"
	chainID, err := ids.FromString(chainIDStr)
	if err != nil {
		t.Fatal(err)
	}

	addr1, err := codec.ParseAddressBech32(mconsts.HRP, "morpheus1qqds2l0ryq5hc2ddps04384zz6rfeuvn3kyvn77hp4n5sv3ahuh6wgkt57y")
	require.NoError(t, err)

	tx := chain.Transaction{
		Base: &chain.Base{
			Timestamp: 1717111222000,
			ChainID:   chainID,
			MaxFee:    uint64(10 * math.Pow(10, 9)),
		},
		Actions: []chain.Action{
			&Transfer{
				To:    addr1,
				Value: 123,
				Memo:  []byte("memo"),
			},
		},
		Auth: nil,
	}

	digest, err := tx.Digest()
	require.NoError(t, err)

	require.Equal(t, hex.EncodeToString(digest), "0000018fcbcdeef0d36e467c73e2840140cc41b3d72f8a5a7446b2399c39b9c74d4cf077d250902400000002540be4000100001b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7000000000000007b000000046d656d6f")

	privBytes, err := codec.LoadHex(
		"323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7",
		ed25519.PrivateKeyLen,
	)
	require.NoError(t, err)

	priv := ed25519.PrivateKey(privBytes)
	factory := auth.NewED25519Factory(priv)

	authRegistry := codec.NewTypeParser[chain.Auth]()
	err = authRegistry.Register((&auth.ED25519{}).GetTypeID(), auth.UnmarshalED25519)
	require.NoError(t, err)

	actionRegistry := codec.NewTypeParser[chain.Action]()
	err = actionRegistry.Register((&Transfer{}).GetTypeID(), UnmarshalTransfer)
	require.NoError(t, err)

	signedTx, err := tx.Sign(factory, actionRegistry, authRegistry)
	require.NoError(t, err)

	p := codec.NewWriter(0, consts.NetworkSizeLimit)
	err = signedTx.Marshal(p)
	require.NoError(t, err)

	signedTxBytes := p.Bytes()

	require.Equal(t, hex.EncodeToString(signedTxBytes), "0000018fcbcdeef0d36e467c73e2840140cc41b3d72f8a5a7446b2399c39b9c74d4cf077d250902400000002540be4000100001b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7000000000000007b000000046d656d6f001b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa739bc9d7a4e74beafcb45cd8fe12200beeb4dac5569407426315eb382d8d11024c5f73200da58f24d8fe9467d86ec0a0c8c2ccb3c15d78e14fd93c66f4a73d802")
}
