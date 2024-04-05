package appenddb

import (
	"os"
	"sync"
)

const stopWrites = 64

// writer attempts to replicate the functionality
// of the bufio.Writer, but never blocks a write on
// a flush to the underlying file.
type writer struct {
	f *os.File

	bufferSize int
	buf        []byte
	buffers    sync.Pool

	writes chan []byte
	done   chan error
}

func newWriter(f *os.File, bufferSize int) *writer {
	w := &writer{
		f: f,

		bufferSize: bufferSize,
		buf:        make([]byte, 0, bufferSize),
		buffers: sync.Pool{
			New: func() interface{} {
				return make([]byte, 0, bufferSize)
			},
		},
		writes: make(chan []byte, stopWrites),
		done:   make(chan error),
	}
	go w.start()
	return w
}

func (w *writer) start() {
	for write := range w.writes {
		if _, err := w.f.Write(write); err != nil {
			w.done <- err
			return
		}
		w.buffers.Put(write)
	}
	w.done <- nil
}

// Write will write the bytes to the buffer concurrently
func (w *writer) Write(p []byte) {
	if len(w.buf)+len(p) > w.bufferSize && len(w.buf) > 0 {
		w.writes <- w.buf
		w.buf = w.buffers.Get().([]byte)
		w.buf = w.buf[:0]
	}
	w.buf = append(w.buf, p...)
}

// Write cannot be called after a call to Flush.
func (w *writer) Flush() error {
	if len(w.buf) > 0 {
		w.writes <- w.buf
	}
	close(w.writes)
	return <-w.done
}
