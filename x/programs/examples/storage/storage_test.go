package storage

import (
	"testing"

	"github.com/ava-labs/avalanchego/ids"
)

func TestXxx(t *testing.T) {
	id := ids.GenerateTestID()
	v := ProgramPrefixKey(id[:], []byte("key"))
	t.Log(v)
	v = ProgramPrefixKey(id[:], []byte("value"))
	t.Log(v)

}
