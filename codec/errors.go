package codec

import "errors"

var (
	ErrTooManyItems       = errors.New("too many items")
	ErrDuplicateItem      = errors.New("duplicate item")
	ErrFieldNotPopulated  = errors.New("field is not populated")
	ErrInvalidBitset      = errors.New("invalid bitset")
	ErrIncorrectHRP       = errors.New("incorrect hrp")
	ErrInsufficientLength = errors.New("insufficient length")
	ErrInvalidSize        = errors.New("invalid size")
)
