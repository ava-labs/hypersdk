package appenddb

import "errors"

var (
	ErrKeyTooLong = errors.New("key too long")
	ErrDuplicate  = errors.New("duplicate key")
)
