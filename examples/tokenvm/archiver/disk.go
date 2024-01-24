package archiver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
)

type DiskArchiverConfig struct {
	Location string `json:"location"`
}

type DiskArchiver struct {
	location string
}

func CreateDiskArchiver(ctx context.Context, configBytes []byte) (*DiskArchiver, error) {
	var conf DiskArchiverConfig
	err := json.Unmarshal(configBytes, &conf)
	if err != nil {
		return nil, err
	}

	locInfo, err := os.Stat(conf.Location)
	if os.IsNotExist(err) {
		err := os.Mkdir(conf.Location, os.ModePerm)
		if err != nil {
			return nil, err
		}
	} else if !locInfo.IsDir() {
		return nil, ErrSpecifiedLocationNotDir
	}

	return &DiskArchiver{location: conf.Location}, nil
}

func (da *DiskArchiver) Put(k []byte, v []byte) error {
	fPath := diskArchiverFilePath(k, da.location)
	err := os.WriteFile(fPath, v, 0664)

	return err
}

func (da *DiskArchiver) Exists(k []byte) (bool, error) {
	fPath := diskArchiverFilePath(k, da.location)

	info, err := os.Stat(fPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if info.IsDir() {
		return true, ErrSpecifiedFileIsADir
	}

	return true, nil
}

func (da *DiskArchiver) Get(k []byte) ([]byte, error) {
	fPath := diskArchiverFilePath(k, da.location)

	data, err := os.ReadFile(fPath)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func diskArchiverFilePath(k []byte, location string) string {
	key := base32EncodeToString(k)
	fPath := filepath.Join(location, key)
	return fPath
}
