package archiver

import (
	"context"
	"encoding/json"
	"strings"
)

type Archiver interface {
	Get(k []byte) ([]byte, error)
	Put(k []byte, v []byte) error
	Exists(k []byte) (bool, error)
}

type ArchiverConfig struct {
	ArchiverType string `json:"archiverType"`
	Enabled      bool   `json:"enabled"`
}

func CreateArchiverByConfig(ctx context.Context, configBytes []byte) (Archiver, error) {
	var config ArchiverConfig
	err := json.Unmarshal(configBytes, &config)
	if err != nil {
		return nil, ErrParsingArchiverConfig
	}

	if !config.Enabled {
		return &NoOpArchiver{}, nil
	}

	switch strings.ToLower(config.ArchiverType) {
	case "aws":
		return CreateS3Archiver(ctx, configBytes)
	case "disk":
		return CreateDiskArchiver(ctx, configBytes)
	default:
		return &NoOpArchiver{}, nil
	}
}
