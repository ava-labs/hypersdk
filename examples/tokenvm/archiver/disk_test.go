package archiver

import (
	"context"
	"encoding/json"
	"testing"
)

func TestDiskReadAndWrite(t *testing.T) {
	conf := DiskArchiverConfig{
		Location: "/tmp/block-storage",
	}

	confBytes, err := json.Marshal(conf)
	if err != nil {
		t.Error(err)
	}

	da, err := CreateDiskArchiver(context.TODO(), confBytes)
	if err != nil {
		t.Error(err)
	}

	key := []byte("blk-1")
	data := []byte("blk data 1")

	exists, err := da.Exists(key)
	t.Logf("%t, %+x", exists, err)
	if err != nil {
		t.Error(err)
	}
	if err == nil && !exists {
		err := da.Put(key, data)
		if err != nil {
			t.Error(err)
		}
	}
	_, err = da.Get(key)
	if err != nil {
		t.Error(err)
	}
}
