package fetcher

import "errors"

var (
	ErrMissingTx       = errors.New("missing transaction")
	ErrInvalidKeyValue = errors.New("invalid key or value")
	ErrStopped         = errors.New("stopped")
)
