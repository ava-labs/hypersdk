// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package list

import (
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"
)

type TestItem struct {
	id        ids.ID
	str       string
	timestamp int64
}

func (mti *TestItem) ID() ids.ID {
	return mti.id
}

func (mti *TestItem) Str() string {
	return mti.str
}

func (mti *TestItem) Expiry() int64 {
	return mti.timestamp
}

func GenerateTestItem(str string) *TestItem {
	id := ids.GenerateTestID()
	return &TestItem{
		id:        id,
		str:       str,
		timestamp: time.Now().UnixMilli(),
	}
}

// Original source: https://gist.github.com/pje/90e727f80685c78a6c1cfff35f62155a
func TestList(t *testing.T) {
	require := require.New(t)
	l := List[*TestItem]{}

	require.Zero(l.root)
	require.Zero(l.Size())
	require.Zero(l.First())
	require.Zero(l.Last())

	l.PushFront(GenerateTestItem("foo"))
	require.Equal(1, l.Size())
	require.Equal("foo", l.First().Value().Str())

	l.PushFront(GenerateTestItem("bar"))
	require.Equal(2, l.Size())
	require.Equal("bar", l.First().Value().Str())
	require.Equal("foo", l.Last().Value().Str())

	l.PushFront(GenerateTestItem("baz"))
	require.Equal(3, l.Size())
	require.Equal("baz", l.First().Value().Str())
	require.Equal("foo", l.Last().Value().Str())

	// list is now (baz, bar, foo)
	require.Equal("bar", l.First().Next().Value().Str())
	require.Nil(l.First().Prev())
	require.Equal("foo", l.First().Next().Next().Value().Str())
	require.Equal("baz", l.First().Next().Prev().Value().Str())

	v := l.Remove(l.First())
	require.Equal(2, l.Size())
	require.Equal("baz", v.Str())
	require.Equal("bar", l.First().Value().Str())
	require.Equal("foo", l.Last().Value().Str())

	v = l.Remove(l.First())
	require.Equal(1, l.Size())
	require.Equal("bar", v.Str())
	require.Equal("foo", l.First().Value().Str())
	require.Equal("foo", l.Last().Value().Str())

	v = l.Remove(l.First())
	require.Equal(0, l.Size())
	require.Equal("foo", v.Str())
	require.Nil(l.First())
	require.Nil(l.Last())
}
