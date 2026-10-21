package storage

import (
	"errors"
	"sync"
)

var ErrWriterClosed = errors.New("persistence writer is closed")

type writeJob struct {
	run  func() error
	done chan error
}

// Writer executes every accepted transaction in order. Jobs are not coalesced:
// pin/delete/expiry acknowledgements must correspond to their actual disk write.
type Writer struct {
	mu     sync.Mutex
	ready  *sync.Cond
	jobs   []writeJob
	closed bool
	done   chan struct{}
}

func NewWriter() *Writer {
	w := &Writer{done: make(chan struct{})}
	w.ready = sync.NewCond(&w.mu)
	go w.run()
	return w
}

func (w *Writer) Submit(fn func() error) <-chan error {
	done := make(chan error, 1)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		done <- ErrWriterClosed
		close(done)
		return done
	}
	w.jobs = append(w.jobs, writeJob{fn, done})
	w.ready.Signal()
	return done
}

func (w *Writer) run() {
	defer close(w.done)
	for {
		w.mu.Lock()
		for len(w.jobs) == 0 && !w.closed {
			w.ready.Wait()
		}
		if len(w.jobs) == 0 {
			w.mu.Unlock()
			return
		}
		job := w.jobs[0]
		w.jobs[0] = writeJob{}
		w.jobs = w.jobs[1:]
		w.mu.Unlock()
		job.done <- job.run()
		close(job.done)
	}
}

// CloseAndWait drains accepted jobs before returning. It is safe to repeat.
func (w *Writer) CloseAndWait() {
	w.mu.Lock()
	w.closed = true
	w.ready.Broadcast()
	w.mu.Unlock()
	<-w.done
}
