package memory

import "fmt"

var (
	ErrOverflow    = fmt.Errorf("overflow")
	ErrUnderflow   = fmt.Errorf("underflow")
	ErrInvalidType = fmt.Errorf("invalid type")
	ErrOutOfBounds = fmt.Errorf("out of bounds")
)
