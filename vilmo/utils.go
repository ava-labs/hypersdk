package vilmo

import "unsafe"

// string2bytes avoid copying the string to a byte slice (which we need
// when writing to disk).
func string2bytes(str string) []byte {
	d := unsafe.StringData(str)
	return unsafe.Slice(d, len(str))
}
