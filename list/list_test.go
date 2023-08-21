package list

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Original source: https://gist.github.com/pje/90e727f80685c78a6c1cfff35f62155a
func TestList(t *testing.T) {
	l := List[string]{}

	assert.Zero(t, l.root)
	assert.Zero(t, l.Len)
	assert.Zero(t, l.First())
	assert.Zero(t, l.Last())

	l.PushFront("foo")
	assert.Equal(t, 1, l.Len)
	assert.Equal(t, "foo", l.First().Value)

	l.PushFront("bar")
	assert.Equal(t, 2, l.Len)
	assert.Equal(t, "bar", l.First().Value)
	assert.Equal(t, "foo", l.Last().Value)

	l.PushFront("baz")
	assert.Equal(t, 3, l.Len)
	assert.Equal(t, "baz", l.First().Value)
	assert.Equal(t, "foo", l.Last().Value)

	// list is now (baz, bar, foo)
	assert.Equal(t, "bar", l.First().Next().Value)
	assert.Nil(t, l.First().Prev())
	assert.Equal(t, "foo", l.First().Next().Next().Value)
	assert.Equal(t, "baz", l.First().Next().Prev().Value)

	v := l.Remove(l.First())
	assert.Equal(t, 2, l.Len)
	assert.Equal(t, "baz", v)
	assert.Equal(t, "bar", l.First().Value)
	assert.Equal(t, "foo", l.Last().Value)

	v = l.Remove(l.First())
	assert.Equal(t, 1, l.Len)
	assert.Equal(t, "bar", v)
	assert.Equal(t, "foo", l.First().Value)
	assert.Equal(t, "foo", l.Last().Value)

	v = l.Remove(l.First())
	assert.Equal(t, 0, l.Len)
	assert.Equal(t, "foo", v)
	assert.Nil(t, l.First())
	assert.Nil(t, l.Last())
}
