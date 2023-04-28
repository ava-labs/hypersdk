package rpc

import "errors"

var (
	ErrTxNotFound    = errors.New("tx not found")
	ErrAssetNotFound = errors.New("asset not found")
)
