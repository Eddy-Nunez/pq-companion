package main

import (
	"log/slog"
	"runtime/debug"
	"sync"
)

// consumerGuard isolates the log-line consumers in dispatchEvent /
// dispatchLine from each other. Without it, a panic in any one of them (a
// nil map entry or bad index on an unusual log line) unwinds the tailer
// goroutine and takes down the whole sidecar, which Electron only reports
// during startup — mid-session the app just silently stops updating.
//
// A consumer that panics is disabled for the rest of the session rather than
// retried: most consumers release their mutex with an explicit Unlock (not
// defer), so a panic can leave it locked, and calling that consumer again
// would deadlock the dispatch goroutine and stall every other consumer too.
type consumerGuard struct {
	mu       sync.Mutex
	disabled map[string]bool
}

func newConsumerGuard() *consumerGuard {
	return &consumerGuard{disabled: map[string]bool{}}
}

// run calls fn unless the consumer called name has already panicked, and
// recovers (logging the stack and disabling it) if fn panics now.
func (g *consumerGuard) run(name string, fn func()) {
	g.mu.Lock()
	off := g.disabled[name]
	g.mu.Unlock()
	if off {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			g.mu.Lock()
			g.disabled[name] = true
			g.mu.Unlock()
			slog.Error("log consumer panicked; disabled until restart",
				"consumer", name, "panic", r, "stack", string(debug.Stack()))
		}
	}()
	fn()
}
