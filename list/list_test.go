// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package list

import (
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/assert"
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
	l := List[*TestItem]{}

	assert.Zero(t, l.root)
	assert.Zero(t, l.Size())
	assert.Zero(t, l.First())
	assert.Zero(t, l.Last())

	l.PushFront(GenerateTestItem("foo"))
	assert.Equal(t, 1, l.Size())
	assert.Equal(t, "foo", l.First().Value().Str())

	l.PushFront(GenerateTestItem("bar"))
	assert.Equal(t, 2, l.Size())
	assert.Equal(t, "bar", l.First().Value().Str())
	assert.Equal(t, "foo", l.Last().Value().Str())

	l.PushFront(GenerateTestItem("baz"))
	assert.Equal(t, 3, l.Size())
	assert.Equal(t, "baz", l.First().Value().Str())
	assert.Equal(t, "foo", l.Last().Value().Str())

	// list is now (baz, bar, foo)
	assert.Equal(t, "bar", l.First().Next().Value().Str())
	assert.Nil(t, l.First().Prev())
	assert.Equal(t, "foo", l.First().Next().Next().Value().Str())
	assert.Equal(t, "baz", l.First().Next().Prev().Value().Str())

	v := l.Remove(l.First())
	assert.Equal(t, 2, l.Size())
	assert.Equal(t, "baz", v.Str())
	assert.Equal(t, "bar", l.First().Value().Str())
	assert.Equal(t, "foo", l.Last().Value().Str())

	v = l.Remove(l.First())
	assert.Equal(t, 1, l.Size())
	assert.Equal(t, "bar", v.Str())
	assert.Equal(t, "foo", l.First().Value().Str())
	assert.Equal(t, "foo", l.Last().Value().Str())

	v = l.Remove(l.First())
	assert.Equal(t, 0, l.Size())
	assert.Equal(t, "foo", v.Str())
	assert.Nil(t, l.First())
	assert.Nil(t, l.Last())
}
