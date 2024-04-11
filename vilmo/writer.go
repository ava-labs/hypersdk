package vilmo

import (
	"os"
	"sync"

	"golang.org/x/sync/errgroup"
)

type writer struct {
	f      *os.File
	offset int64

	errg *errgroup.Group
	pool sync.Pool

	bufferSize int
	buf        []byte
	pos        int
}

func newWriter(f *os.File, start int64, bufferSize int) *writer {
	w := &writer{f: f, offset: start, errg: &errgroup.Group{}, bufferSize: bufferSize}
	w.pool.New = func() interface{} {
		// Pool must return pointers to actually avoid memory allocations
		b := make([]byte, bufferSize)
		return &b
	}
	pbuf := w.pool.Get().(*[]byte)
	w.buf = (*pbuf)
	return w
}

func (w *writer) Write(data []byte) {
	dataPos := 0
	for dataPos < len(data) {
		// Add data we can write to buffer
		space := w.bufferSize - w.pos
		writable := min(space, len(data)-dataPos)
		copy(w.buf[w.pos:], data[dataPos:dataPos+writable])
		w.pos += writable
		dataPos += writable

		// If the buffer is full, write it to disk at offset
		if w.pos == w.bufferSize {
			buf := w.buf
			offset := w.offset
			w.errg.Go(func() error {
				_, err := w.f.WriteAt(buf, offset)
				w.pool.Put(&buf)
				return err
			})
			w.offset += int64(w.pos)
			w.pos = 0

			// Attempt to reuse buffer bytes once previous writes complete
			pbuf := w.pool.Get().(*[]byte)
			w.buf = (*pbuf)
		}
	}
}

func (w *writer) Flush() error {
	// If we have written to the current buffer, write it to disk
	if w.pos > 0 {
		w.errg.Go(func() error {
			_, err := w.f.WriteAt(w.buf[:w.pos], w.offset)
			return err
		})
	}
	return w.errg.Wait()
}
