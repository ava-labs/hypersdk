package archiver

var _ Archiver = (*NoOpArchiver)(nil)

type NoOpArchiver struct {
}

func (*NoOpArchiver) Get(k []byte) ([]byte, error) {
	return nil, ErrNoopArchiver
}

func (*NoOpArchiver) Put(k []byte, v []byte) error {
	return ErrNoopArchiver
}

func (*NoOpArchiver) Exists(k []byte) (bool, error) {
	return false, ErrNoopArchiver
}
