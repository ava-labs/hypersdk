package host

type Import interface {
	// Name returns the name of this import module.
	Name() string
	// Register registers this import module with the provided link. 
	Register(*Link) error
}
