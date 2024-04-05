package appenddb

import "errors"

var (
	ErrKeyTooLong  = errors.New("key too long")
	ErrNotPrepared = errors.New("not prepared")
	ErrCorrupt     = errors.New("corrupt data")
)
