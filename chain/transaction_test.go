// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
)

var (
	_ chain.Action = (*mockTransferAction)(nil)
	_ chain.Action = (*action2)(nil)
)

func TestTransactionTestSuite(t *testing.T) {
	suite.Run(t, new(TransactionTestSuite))
}

type TransactionTestSuite struct {
	suite.Suite

	privateKey  ed25519.PrivateKey
	factory     *auth.ED25519Factory
	actionCodec *codec.TypeParser[chain.Action]
	authCodec   *codec.TypeParser[chain.Auth]
}

func (s *TransactionTestSuite) SetupTest() {
	require := require.New(s.T())
	var err error
	s.privateKey, err = ed25519.GeneratePrivateKey()
	require.NoError(err)
	s.factory = auth.NewED25519Factory(s.privateKey)

	s.actionCodec = codec.NewTypeParser[chain.Action]()
	s.authCodec = codec.NewTypeParser[chain.Auth]()
	err = s.authCodec.Register(&auth.ED25519{}, auth.UnmarshalED25519)
	require.NoError(err)
	err = s.actionCodec.Register(&mockTransferAction{}, unmarshalTransfer)
	require.NoError(err)
	err = s.actionCodec.Register(&action2{}, unmarshalAction2)
	require.NoError(err)
}

func (s *TransactionTestSuite) TestJSONMarshalUnmarshal() {
	require := require.New(s.T())
	txData := getTransactionData()

	signedTx, err := txData.Sign(s.factory)
	require.NoError(err)

	b, err := json.Marshal(signedTx)
	require.NoError(err)
	parser := chaintest.NewParser(nil, s.actionCodec, s.authCodec, nil)
	var txFromJSON chain.Transaction
	err = txFromJSON.UnmarshalJSON(b, parser)
	require.NoError(err)
	require.Equal(signedTx.Bytes(), txFromJSON.Bytes())
}

// TestMarshalUnmarshalTransactionData roughly validates that a transaction data packs and unpacks correctly.
func (s *TransactionTestSuite) TestMarshalUnmarshalTransactionData() {
	require := require.New(s.T())
	tx := getTransactionData()
	writerPacker := codec.NewWriter(0, consts.NetworkSizeLimit)
	err := tx.Marshal(writerPacker)
	require.NoError(err)
	txDataBytes, err := tx.UnsignedBytes()
	require.NoError(err)
	require.Equal(writerPacker.Bytes(), txDataBytes)
	readerPacker := codec.NewReader(writerPacker.Bytes(), consts.NetworkSizeLimit)
	unmarshaledTxData, err := chain.UnmarshalTxData(readerPacker, s.actionCodec)
	require.NoError(err)
	require.Equal(tx, *unmarshaledTxData)
}

func (s *TransactionTestSuite) TestSignRawActionBytesTx() {
	require := require.New(s.T())
	tx := getTransactionData()

	signedTx, err := tx.Sign(s.factory)
	require.NoError(err)

	p := codec.NewWriter(0, consts.NetworkSizeLimit)
	require.NoError(signedTx.Actions.MarshalInto(p))
	actionsBytes := p.Bytes()
	rawSignedTxBytes, err := chain.SignRawActionBytesTx(tx.Base, actionsBytes, s.factory)
	require.NoError(err)
	require.Equal(signedTx.Bytes(), rawSignedTxBytes)
}

// TestMarshalUnmarshalTransaction roughly validates that a transaction packs and unpacks correctly.
func (s *TransactionTestSuite) TestMarshalUnmarshalTransaction() {
	require := require.New(s.T())
	tx := getTransactionData()
	// call UnsignedBytes so that the "unsignedBytes" field would get populated.
	txBeforeSignBytes, err := tx.UnsignedBytes()
	require.NoError(err)

	signedTx, err := tx.Sign(s.factory)
	require.NoError(err)
	unsignedTxAfterSignBytes, err := signedTx.TransactionData.UnsignedBytes()
	require.NoError(err)
	require.Equal(txBeforeSignBytes, unsignedTxAfterSignBytes)
	require.NotNil(signedTx.Auth)
	require.Equal(len(signedTx.Actions), len(tx.Actions))
	for i, action := range signedTx.Actions {
		require.Equal(tx.Actions[i], action)
	}
	writerPacker := codec.NewWriter(0, consts.NetworkSizeLimit)
	err = signedTx.Marshal(writerPacker)
	require.NoError(err)
	require.Equal(signedTx.ID(), utils.ToID(writerPacker.Bytes()))
	require.Equal(signedTx.Bytes(), writerPacker.Bytes())

	unsignedTxBytes, err := signedTx.UnsignedBytes()
	require.NoError(err)
	originalUnsignedTxBytes, err := tx.UnsignedBytes()
	require.NoError(err)

	require.Equal(unsignedTxBytes, originalUnsignedTxBytes)
	require.Len(unsignedTxBytes, 168)

	readerPacker := codec.NewReader(writerPacker.Bytes(), consts.NetworkSizeLimit)
	unmarshaledTx, err := chain.UnmarshalTx(readerPacker, s.actionCodec, s.authCodec)
	require.NoError(err)
	require.Equal(writerPacker.Bytes(), unmarshaledTx.Bytes())
}

func getTransactionData() chain.TransactionData {
	return chain.TransactionData{
		Base: &chain.Base{
			Timestamp: 1724315246000,
			ChainID:   [32]byte{1, 2, 3, 4, 5, 6, 7},
			MaxFee:    1234567,
		},
		Actions: []chain.Action{
			&mockTransferAction{
				To:    codec.Address{1, 2, 3, 4},
				Value: 4,
				Memo:  []byte("hello"),
			},
			&mockTransferAction{
				To:    codec.Address{4, 5, 6, 7},
				Value: 123,
				Memo:  []byte("world"),
			},
			&action2{
				A: 2,
				B: 4,
			},
		},
	}
}

type abstractMockAction struct{}

func (*abstractMockAction) ComputeUnits(chain.Rules) uint64 {
	panic("unimplemented")
}

func (*abstractMockAction) Execute(_ context.Context, _ chain.Rules, _ state.Mutable, _ int64, _ codec.Address, _ ids.ID) (codec.Typed, error) {
	panic("unimplemented")
}

func (*abstractMockAction) StateKeys(_ codec.Address, _ ids.ID) state.Keys {
	panic("unimplemented")
}

func (*abstractMockAction) ValidRange(chain.Rules) (start int64, end int64) {
	panic("unimplemented")
}

type mockTransferAction struct {
	abstractMockAction
	To    codec.Address `serialize:"true" json:"to"`
	Value uint64        `serialize:"true" json:"value"`
	Memo  []byte        `serialize:"true" json:"memo"`
}

func (*mockTransferAction) GetTypeID() uint8 {
	return 111
}

type action2 struct {
	abstractMockAction
	A uint64 `serialize:"true" json:"a"`
	B uint64 `serialize:"true" json:"b"`
}

func (*action2) GetTypeID() uint8 {
	return 222
}

func unmarshalTransfer(p *codec.Packer) (chain.Action, error) {
	var transfer mockTransferAction
	err := codec.LinearCodec.UnmarshalFrom(p.Packer, &transfer)
	return &transfer, err
}

func unmarshalAction2(p *codec.Packer) (chain.Action, error) {
	var action action2
	err := codec.LinearCodec.UnmarshalFrom(p.Packer, &action)
	return &action, err
}
