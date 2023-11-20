package imports

// Import defines host functions exposed by this runtime that can be imported by
// a guest module.
type Import interface {
	// Name returns the name of this import module.
	Name() string
	// Instantiate instantiates an all of the functions exposed by this import module.
	Register(Supported) error
}
