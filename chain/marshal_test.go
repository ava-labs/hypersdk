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
	size1, err := GetSize(obj)
	require.NoError(err)
	require.Equal(consts.Uint8Len, size1)

	mockObjectSize := 100000
	objMarshaler := &mockObjectMarshaler{
		mockSize: mockObjectSize,
	}
	size2, err := GetSize(objMarshaler)
	require.NoError(err)
	require.Equal(mockObjectSize, size2)
}

func TestMarshalActionInto(t *testing.T) {
	require := require.New(t)

	obj := &mockObject{Value: 7}
	p1 := codec.NewWriter(0, consts.NetworkSizeLimit)
	err := marshalInto(obj, p1)
	require.NoError(err)
	require.Equal([]byte{7}, p1.Bytes())

	mockBytes := []byte("hello")
	objMarshaler := &mockObjectMarshaler{
		mockBytes: mockBytes,
	}
	p2 := codec.NewWriter(0, consts.NetworkSizeLimit)
	err = marshalInto(objMarshaler, p2)
	require.NoError(err)
	require.Equal(mockBytes, p2.Bytes())
}
