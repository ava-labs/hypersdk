package dsmr

// Verifier provides a generic interface for verifying an item
// within a given context.
type Verifier[Context any, T any] interface {
	Verify(Context, T) error
}
