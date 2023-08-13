// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package workers

import (
	"sync"
)

var (
	_ Workers = (*ParallelWorkers)(nil)
	_ Job     = (*ParallelJob)(nil)
)

// ParallelWorkers is a struct representing a workers pool.
//
// Limit number of concurrent goroutines with resetable error
// Ensure minimal overhead for parallel ops
type ParallelWorkers struct {
	count int               // Number of workers in the pool
	queue chan *ParallelJob // A channel for passing jobs

	// tracking state
	lock              sync.RWMutex
	shouldShutdown    bool
	triggeredShutdown bool

	// single job execution
	err   error // requires lock
	sg    sync.WaitGroup
	tasks chan func() error

	// shutdown coordination
	ackShutdown    chan struct{}
	stopWorkers    chan struct{}
	stoppedWorkers chan struct{}
}

// Goroutines allocate a minimum of 2KB of memory, we can save this by reusing
// the context. This is especially useful if the goroutine stack is expanded
// during use.
//
// Current size: https://github.com/golang/go/blob/fa463cc96d797c218be4e218723f83be47e814c8/src/runtime/stack.go#L74-L75
// Backstory: https://medium.com/a-journey-with-go/go-how-does-the-goroutine-stack-size-evolve-447fc02085e5
func NewParallel(workers int, maxJobs int) Workers {
	w := &ParallelWorkers{
		count: workers,
		queue: make(chan *ParallelJob, maxJobs),

		tasks:          make(chan func() error),
		ackShutdown:    make(chan struct{}),
		stopWorkers:    make(chan struct{}),
		stoppedWorkers: make(chan struct{}),
	}
	w.processQueue()
	for i := 0; i < workers; i++ {
		w.startWorker()
	}
	return w
}

// processQueue starts a new goroutine that listens to the queue channel for jobs.
// It assigns that jobs' tasks to w until shouldShutdown is set.
func (w *ParallelWorkers) processQueue() {
	go func() {
		for j := range w.queue {
			// Don't do work if should shutdown
			w.lock.Lock()
			shouldShutdown := w.shouldShutdown
			w.lock.Unlock()
			if shouldShutdown {
				j.result <- ErrShutdown
				continue
			}
			// Process tasks
			for t := range j.tasks {
				w.sg.Add(1)
				w.tasks <- t
			}
			w.sg.Wait()
			// Send result to queue and reset err
			w.lock.Lock()
			close(j.completed)
			j.result <- w.err
			w.err = nil
			w.lock.Unlock()
		}
		// Ensure stop returns
		w.lock.Lock()
		if w.shouldShutdown && !w.triggeredShutdown {
			w.triggeredShutdown = true
			close(w.ackShutdown)
		}
		w.lock.Unlock()
	}()
}

// startWorker starts a new goroutine that listens to two channels.
// The stopWorkers channel signals the worker to stop processing tasks.
// The tasks channel attempts to process a job.
func (w *ParallelWorkers) startWorker() {
	go func() {
		for {
			select {
			case <-w.stopWorkers:
				w.stoppedWorkers <- struct{}{}
				return
			case j := <-w.tasks:
				// Check if we should even do the work
				w.lock.RLock()
				err := w.err
				w.lock.RUnlock()
				if err != nil {
					w.sg.Done()
					return
				}
				// Attempt to process the job
				if err := j(); err != nil {
					w.lock.Lock()
					if w.err == nil {
						w.err = err
					}
					w.lock.Unlock()
				}
				w.sg.Done()
			}
		}
	}()
}

// Stop stops the worker pool by setting shouldShutdown, closing the
// queue and waiting for all workers to complete.
func (w *ParallelWorkers) Stop() {
	w.lock.Lock()
	w.shouldShutdown = true
	w.lock.Unlock()
	close(w.queue)

	// Wait for scheduler to return
	<-w.ackShutdown
	close(w.stopWorkers)

	// Wait for all workers to return
	for i := 0; i < w.count; i++ {
		<-w.stoppedWorkers
	}
}

type ParallelJob struct {
	count     int
	tasks     chan func() error
	completed chan struct{}
	result    chan error
}

// Go adds [f] to the j's task channel.
func (j *ParallelJob) Go(f func() error) {
	j.tasks <- f
}

// Done closes the tasks channel, then waits for the job's completed channel
// to be closed. Calls [f] after the tasks have been completed if [f] is not null.
func (j *ParallelJob) Done(f func()) {
	close(j.tasks)
	if f != nil {
		// Callback when completed (useful for tracing)
		go func() {
			<-j.completed
			f()
		}()
	}
}

// Wait returns the value received by the j.result channel. This only occurs
// once all tasks for a job are completed and after j.Done has been called.
func (j *ParallelJob) Wait() error {
	return <-j.result
}

// Workers returns the number of workers that will execute the job. This can be used
// by callers to generate batch sizes that lead to the most efficient computation.
func (j *ParallelJob) Workers() int {
	return j.count
}

// NewJob creates a new job and adds it to the workers' queue. [taskBacklog]
// specifies the maximum number of tasks that can be added to the created job
// channel.
//
// If you don't want to block, make sure taskBacklog is greater than all
// possible tasks you'll add.
func (w *ParallelWorkers) NewJob(taskBacklog int) (Job, error) {
	w.lock.Lock()
	shouldShutdown := w.shouldShutdown
	w.lock.Unlock()
	if shouldShutdown {
		return nil, ErrShutdown
	}
	j := &ParallelJob{
		count:     w.count,
		tasks:     make(chan func() error, taskBacklog),
		completed: make(chan struct{}),
		result:    make(chan error, 1),
	}
	w.queue <- j
	return j, nil
}
