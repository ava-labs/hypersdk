package vilmo

import "errors"

var (
	ErrKeyTooLong = errors.New("key too long")
	ErrCorrupt    = errors.New("corrupt data")
)
