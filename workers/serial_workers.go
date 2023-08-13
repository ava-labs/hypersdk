package workers

import "sync"

var (
	_ Workers = (*SerialWorkers)(nil)
	_ Job     = (*SerialJob)(nil)
)

type SerialWorkers struct{}

func NewSerial() Workers {
	return &SerialWorkers{}
}

type SerialJob struct {
	once sync.Once
	err  error
}

func (*SerialWorkers) NewJob(_ int) (Job, error) {
	return &SerialJob{}, nil
}

func (*SerialWorkers) Stop() {}

func (j *SerialJob) Go(f func() error) {
	if j.err != nil {
		return
	}
	if err := f(); err != nil {
		j.once.Do(func() {
			j.err = err
		})
	}
}

func (*SerialJob) Done(f func()) {
	f()
}

func (j *SerialJob) Wait() error {
	return j.err
}

func (*SerialJob) Workers() int {
	return 1
}
