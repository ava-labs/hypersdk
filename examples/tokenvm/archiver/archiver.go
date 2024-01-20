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

func CreateArchiverByConfig(ctx context.Context, b []byte) (Archiver, error) {
	var config ArchiverConfig
	err := json.Unmarshal(b, &config)
	if err != nil {
		return nil, errParsingArchiverConfig
	}

	if !config.Enabled {
		return &NoOpArchiver{}, nil
	}

	switch strings.ToLower(config.ArchiverType) {
	case "aws":
		return CreateS3Archiver(ctx, b)
	default:
		return &NoOpArchiver{}, nil
	}
}
