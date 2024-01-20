package archiver

var _ Archiver = (*NoOpArchiver)(nil)

type NoOpArchiver struct {
}

func (*NoOpArchiver) Get(k []byte) ([]byte, error) {
	return nil, nil
}

func (*NoOpArchiver) Put(k []byte, v []byte) error {
	return nil
}

func (*NoOpArchiver) Exists(k []byte) (bool, error) {
	return false, nil
}
