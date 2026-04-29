// Package lifecycle provides a small ordered-shutdown coordinator for the
// backend binaries. Subsystems register Stop hooks during initialization;
// on shutdown the hooks are invoked in reverse-registration order, each
// with its own timeout, so a stuck or slow subsystem cannot block the rest
// of the teardown.
package lifecycle

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/klog/v2"
)

// Hook is a single named teardown step. Stop is called with a context whose
// deadline is the smaller of the per-hook Timeout and the parent shutdown
// deadline. A zero Timeout means "no per-hook ceiling beyond the parent
// deadline".
type Hook struct {
	Name    string
	Stop    func(ctx context.Context) error
	Timeout time.Duration
}

// Coordinator collects shutdown hooks and runs them in reverse-registration
// order. It is safe to register hooks concurrently with other goroutines,
// but Run should be called exactly once from a single shutdown driver
// goroutine.
type Coordinator struct {
	mu    sync.Mutex
	hooks []Hook
}

// New returns an empty Coordinator.
func New() *Coordinator { return &Coordinator{} }

// Add registers a hook. Hooks added later run first during Run (LIFO).
// A nil stop function is silently dropped to make conditional registration
// (e.g. "register only when feature X is enabled") cheaper at call sites.
func (c *Coordinator) Add(name string, timeout time.Duration, stop func(ctx context.Context) error) {
	if stop == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hooks = append(c.hooks, Hook{Name: name, Stop: stop, Timeout: timeout})
}

// Run executes every registered hook in reverse-registration order. The
// parent ctx caps total shutdown time; each hook additionally honors its
// own Timeout. Errors and panics are logged and never abort the chain so a
// single broken subsystem cannot prevent others from cleaning up.
func (c *Coordinator) Run(ctx context.Context) {
	c.mu.Lock()
	hooks := make([]Hook, len(c.hooks))
	copy(hooks, c.hooks)
	c.mu.Unlock()

	klog.Infof("lifecycle: running %d shutdown hook(s)", len(hooks))
	for i := len(hooks) - 1; i >= 0; i-- {
		h := hooks[i]
		if ctx.Err() != nil {
			klog.Warningf("lifecycle: parent deadline reached, skipping remaining hook %q (and earlier)", h.Name)
			return
		}
		runHook(ctx, h)
	}
	klog.Infof("lifecycle: all shutdown hooks done")
}

func runHook(parent context.Context, h Hook) {
	hookCtx := parent
	var cancel context.CancelFunc
	if h.Timeout > 0 {
		hookCtx, cancel = context.WithTimeout(parent, h.Timeout)
		defer cancel()
	}

	start := time.Now()
	err := safeStop(hookCtx, h.Stop)
	elapsed := time.Since(start)
	switch {
	case err == nil:
		klog.Infof("lifecycle: hook %q stopped in %s", h.Name, elapsed)
	default:
		klog.Errorf("lifecycle: hook %q failed after %s: %v", h.Name, elapsed, err)
	}
}

func safeStop(ctx context.Context, stop func(context.Context) error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic during shutdown: %v", r)
		}
	}()
	return stop(ctx)
}
