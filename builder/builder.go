package builder

import "context"

type Builder interface {
	Run()
	Queue(context.Context) // new tx, block verified, post-block build (if mempool > 0)
	Force(context.Context) error
	Done() // wait after stop
}
