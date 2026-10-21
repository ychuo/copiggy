package app

import (
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func awaitDispatcher(t *testing.T, d *dispatcher) {
	t.Helper()
	select {
	case <-d.done:
	case <-time.After(5 * time.Second):
		t.Fatal("dispatcher did not drain")
	}
}

func TestDispatcherSealDrainsAcceptedCommandsBeforeFinal(t *testing.T) {
	d := newDispatcher()
	t.Cleanup(d.close)
	started, release := make(chan struct{}), make(chan struct{})
	var order []int
	d.submit(func() {
		close(started)
		<-release
		order = append(order, 0)
	})
	<-started
	for i := 1; i <= 20; i++ {
		if !d.submit(func() { order = append(order, i) }) {
			t.Fatal("running dispatcher rejected a command")
		}
	}
	if !d.seal(func() { order = append(order, 21) }) {
		t.Fatal("first shutdown transaction was rejected")
	}
	if d.submit(func() { order = append(order, 99) }) {
		t.Fatal("sealed dispatcher accepted a late command")
	}
	if d.seal(func() { order = append(order, 100) }) {
		t.Fatal("dispatcher accepted a second shutdown transaction")
	}
	close(release)
	awaitDispatcher(t, d)
	want := make([]int, 22)
	for i := range want {
		want[i] = i
	}
	if !slices.Equal(order, want) {
		t.Fatalf("commands or final transaction ran out of order: %v", order)
	}
}

func TestDispatcherConcurrentAdmissionIsSerialAndFullyDrained(t *testing.T) {
	d := newDispatcher()
	t.Cleanup(d.close)
	const count = 64
	var admissions sync.WaitGroup
	var running, completed atomic.Int32
	var overlapped atomic.Bool
	for range count {
		admissions.Add(1)
		go func() {
			defer admissions.Done()
			if !d.submit(func() {
				if running.Add(1) != 1 {
					overlapped.Store(true)
				}
				completed.Add(1)
				running.Add(-1)
			}) {
				t.Error("command rejected before shutdown")
			}
		}()
	}
	admissions.Wait()
	var finalCount int32
	d.seal(func() { finalCount = completed.Load() })
	awaitDispatcher(t, d)
	if overlapped.Load() || finalCount != count {
		t.Fatalf("shutdown raced accepted work: concurrent=%v completed=%d", overlapped.Load(), finalCount)
	}
}

func TestDispatcherSealFromCommandDoesNotBlockWorker(t *testing.T) {
	d := newDispatcher()
	t.Cleanup(d.close)
	var finalRan bool
	d.submit(func() {
		if !d.seal(func() { finalRan = true }) {
			t.Error("worker could not seal its own queue")
		}
	})
	awaitDispatcher(t, d)
	if !finalRan {
		t.Fatal("final transaction did not run")
	}
}
