package loadgen

import "errors"

var (
	ErrTxFailed       = errors.New("tx failed on-chain")
	ErrInvalidKeyType = errors.New("invalid key type")
)
