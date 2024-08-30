// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
)

type mockObject struct {
	Value uint8 `serialize:"true"`
}

type mockObjectMarshaler struct {
	mockSize  int
	mockBytes []byte
}

var _ Marshaler = (*mockObjectMarshaler)(nil)

func (m *mockObjectMarshaler) Size() int {
	return m.mockSize
}

func (m *mockObjectMarshaler) Marshal(p *codec.Packer) {
	p.PackFixedBytes(m.mockBytes)
}

func TestGetActionSize(t *testing.T) {
	require := require.New(t)

	obj := &mockObject{Value: 7}
	size1, err := getSize(obj)
	require.NoError(err)
	require.Equal(consts.Uint8Len, size1)

	objMarshaler := &mockObjectMarshaler{
		mockSize: 100000,
	}
	size2, err := getSize(objMarshaler)
	require.NoError(err)
	require.Equal(100000, size2)
}

func TestMarshalActionInto(t *testing.T) {
	require := require.New(t)

	obj := &mockObject{Value: 7}
	p1 := codec.NewWriter(0, consts.NetworkSizeLimit)
	err := marshalInto(obj, p1)
	require.NoError(err)
	require.Equal([]byte{7}, p1.Bytes())

	objMarshaler := &mockObjectMarshaler{
		mockBytes: []byte{1, 2, 3, 4, 5},
	}
	p2 := codec.NewWriter(0, consts.NetworkSizeLimit)
	err = marshalInto(objMarshaler, p2)
	require.NoError(err)
	require.Equal([]byte{1, 2, 3, 4, 5}, p2.Bytes())
}
