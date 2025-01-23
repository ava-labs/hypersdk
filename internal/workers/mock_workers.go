// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package workers

var (
	_ Workers = (*MockWorkers)(nil)
	_ Job     = (*MockJob)(nil)
)

type MockWorkers struct {
	OnNewJob func(backlog int) (Job, error)
}

func (m *MockWorkers) NewJob(backlog int) (Job, error) {
	return m.OnNewJob(backlog)
}

func (*MockWorkers) Stop() {}

type MockJob struct {
	OnWaitError error
}

func (*MockJob) Done(func()) {}

func (*MockJob) Go(func() error) {}

func (m *MockJob) Wait() error {
	return m.OnWaitError
}

func (*MockJob) Workers() int {
	panic("unimplemented")
}
