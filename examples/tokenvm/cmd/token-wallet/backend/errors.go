package backend

import "errors"

var (
	ErrDuplicate    = errors.New("duplicate")
	ErrAssetMissing = errors.New("asset missing")
)
