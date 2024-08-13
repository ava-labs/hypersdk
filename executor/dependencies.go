package executor

type Metrics interface {
	RecordBlocked()
	RecordExecutable()
}
