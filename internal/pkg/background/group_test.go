package background

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestGroup(t *testing.T) {
	g := New()
	var finished atomic.Bool
	started := make(chan struct{})
	if !g.Go(func(ctx context.Context) {
		close(started)
		<-ctx.Done() // a job that runs until shutdown cancels it
		time.Sleep(20 * time.Millisecond)
		finished.Store(true)
	}) {
		t.Fatal("Go refused before Stop")
	}
	<-started
	if g.Stopping() {
		t.Fatal("stopping before Stop")
	}
	if !g.Stop(time.Second) || !finished.Load() {
		t.Fatal("Stop returned before the job finished")
	}
	if !g.Stopping() {
		t.Fatal("not stopping after Stop")
	}
	if g.Go(func(context.Context) { t.Error("ran after Stop") }) {
		t.Fatal("Go accepted work after Stop")
	}

	// A job that ignores cancellation makes Stop give up at the timeout.
	stuck := New()
	release := make(chan struct{})
	defer close(release)
	stuck.Go(func(context.Context) { <-release })
	if stuck.Stop(20 * time.Millisecond) {
		t.Fatal("Stop reported done while a job was still running")
	}
}
func TestGroupRecoversPanic(t *testing.T) {
	g := New()
	if !g.Go(func(ctx context.Context) {
		panic("test panic")
	}) {
		t.Fatal("Go refused before Stop")
	}
	if !g.Stop(time.Second) {
		t.Fatal("Stop returned false after a panic")
	}
	if !g.Stopping() {
		t.Fatal("not stopping after Stop")
	}
	if g.Go(func(context.Context) { t.Error("ran after panic Stop") }) {
		t.Fatal("Go accepted work after Stop")
	}

	// A separate group still works normally after a recovered panic.
	g2 := New()
	var finished atomic.Bool
	if !g2.Go(func(ctx context.Context) { finished.Store(true) }) {
		t.Fatal("Go refused on a new group")
	}
	if !g2.Stop(time.Second) || !finished.Load() {
		t.Fatal("Stop didn't finish the normal job")
	}
}
