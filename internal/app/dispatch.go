package app

import "sync"

// dispatcher is a FIFO command owner. Submit never waits for I/O on AppKit's
// thread, and completion runs only after the command's durable outcome is known.
type dispatcher struct {
	mu     sync.Mutex
	jobs   []func()
	wake   chan struct{}
	done   chan struct{}
	closed bool
}

func newDispatcher() *dispatcher {
	d := &dispatcher{wake: make(chan struct{}, 1), done: make(chan struct{})}
	go func() {
		defer close(d.done)
		for range d.wake {
			for {
				d.mu.Lock()
				if len(d.jobs) == 0 {
					closed := d.closed
					d.mu.Unlock()
					if closed {
						return
					}
					break
				}
				job := d.jobs[0]
				d.jobs[0] = nil
				d.jobs = d.jobs[1:]
				d.mu.Unlock()
				job()
			}
		}
	}()
	return d
}
func (d *dispatcher) submit(job func()) bool {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return false
	}
	d.jobs = append(d.jobs, job)
	d.mu.Unlock()
	select {
	case d.wake <- struct{}{}:
	default:
	}
	return true
}

// seal closes admission and appends the shutdown transaction atomically. Every
// accepted command runs before final; no command can reach a closed repository
// by arriving between the last persistence job and application termination.
func (d *dispatcher) seal(final func()) bool {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return false
	}
	d.closed = true
	if final != nil {
		d.jobs = append(d.jobs, final)
	}
	d.mu.Unlock()
	select {
	case d.wake <- struct{}{}:
	default:
	}
	return true
}

func (d *dispatcher) close() { d.seal(nil) }
