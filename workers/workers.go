package workers

type Workers interface {
	NewJob(backlog int) (Job, error)
	Stop()
}

type Job interface {
	Go(func() error)
	Done(func())
	Wait() error
	Workers() int
}
