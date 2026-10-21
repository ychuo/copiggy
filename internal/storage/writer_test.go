package storage

import (
	"errors"
	"reflect"
	"testing"
)

func TestWriterPreservesEveryTransactionOrderAndCompletion(t *testing.T) {
	w := NewWriter()
	entered, release := make(chan struct{}), make(chan struct{})
	var order []int
	first := w.Submit(func() error { close(entered); <-release; order = append(order, 1); return nil })
	<-entered
	failure := errors.New("write failure")
	second := w.Submit(func() error { order = append(order, 2); return failure })
	third := w.Submit(func() error { order = append(order, 3); return nil })
	select {
	case <-first:
		t.Fatal("completion reported before actual write")
	default:
	}
	close(release)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if err := <-second; !errors.Is(err, failure) {
		t.Fatal("write error not reported")
	}
	if err := <-third; err != nil {
		t.Fatal(err)
	}
	w.CloseAndWait()
	if !reflect.DeepEqual(order, []int{1, 2, 3}) {
		t.Fatalf("write order changed: %v", order)
	}
	if err := <-w.Submit(func() error { t.Fatal("closed writer ran job"); return nil }); !errors.Is(err, ErrWriterClosed) {
		t.Fatal("closed writer accepted job")
	}
	w.CloseAndWait()
}

func TestWriterCloseDrainsAcceptedJobs(t *testing.T) {
	w := NewWriter()
	entered, release := make(chan struct{}), make(chan struct{})
	first := w.Submit(func() error { close(entered); <-release; return nil })
	<-entered
	secondRan := false
	second := w.Submit(func() error { secondRan = true; return nil })
	closed := make(chan struct{})
	go func() { w.CloseAndWait(); close(closed) }()
	select {
	case <-closed:
		t.Fatal("close did not wait for accepted work")
	default:
	}
	close(release)
	<-closed
	if !secondRan {
		t.Fatal("close discarded queued transaction")
	}
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if err := <-second; err != nil {
		t.Fatal(err)
	}
}
