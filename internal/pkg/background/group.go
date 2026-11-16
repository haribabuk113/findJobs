// Package background runs work that outlives a request (exchange-rate
// refreshes, asset publishing, later the scheduled jobs) but must stop before
// the app closes its database pool on shutdown.
package background

import (
	"context"
	"runtime/debug"
	"sync"
	"time"

	"findJobs/internal/pkg/logger"
)

// Group tracks background goroutines. Stop cancels their context and waits
// for them, so none is cut off mid-transaction by the pool closing.
type Group struct {
	// ctx is the group's lifetime context: built in New from context.Background
	// and cancelled by Stop. Storing it here is deliberate — every job must
	// share one context so a single Stop cancels them all. It is app-scoped
	// (not request-scoped) and its lifetime is the Group's own, so the usual
	// "never store a context" rule does not apply.
	ctx    context.Context
	cancel context.CancelFunc

	mu      sync.Mutex // guards stopped, so no Go can Add after Stop starts waiting
	stopped bool
	wg      sync.WaitGroup
}

func New() *Group {
	ctx, cancel := context.WithCancel(context.Background())
	return &Group{ctx: ctx, cancel: cancel}
}

// Go runs fn in a goroutine with the group's context, which Stop cancels.
// Once Stop has begun it runs nothing and returns false.
func (g *Group) Go(fn func(ctx context.Context)) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.stopped {
		return false
	}

	g.wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				l := logger.Get()
				l.Error().Interface("panic", r).Str("stack", string(debug.Stack())).Msg("background goroutine panicked")
			}
			g.wg.Done()
		}()
		fn(g.ctx)
	}()
	return true
}

// Stopping reports whether Stop has cancelled the work, so a job can log a
// shutdown cancellation as expected instead of as a failure.
func (g *Group) Stopping() bool { return g.ctx.Err() != nil }

// Stop cancels the work and waits up to timeout for it to return. It returns
// false when something was still running at the deadline.
func (g *Group) Stop(timeout time.Duration) bool {
	g.mu.Lock()
	g.stopped = true
	g.mu.Unlock()
	g.cancel()

	done := make(chan struct{})
	go func() {
		g.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}
