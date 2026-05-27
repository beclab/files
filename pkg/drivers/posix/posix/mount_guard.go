package posix

import (
	"files/pkg/common"
	"files/pkg/global"
	"files/pkg/models"
	"fmt"
	"sync"
	"time"
)

const (
	defaultMountProbeTimeout = 8 * time.Second
	defaultMountCooldown     = 45 * time.Second
	defaultMountProbeLease   = 30 * time.Second
)

type mountGuardState struct {
	probing       bool
	probeStarted  time.Time
	cooldownUntil time.Time
	generation    uint64
	seenInvalid   bool
	lastInvalid   bool
}

type mountGuard struct {
	mu           sync.Mutex
	probeTimeout time.Duration
	cooldown     time.Duration
	probeLease   time.Duration
	states       map[string]*mountGuardState
}

func newMountGuard(probeTimeout, cooldown, probeLease time.Duration) *mountGuard {
	return &mountGuard{
		probeTimeout: probeTimeout,
		cooldown:     cooldown,
		probeLease:   probeLease,
		states:       make(map[string]*mountGuardState),
	}
}

var externalMountGuard = newMountGuard(defaultMountProbeTimeout, defaultMountCooldown, defaultMountProbeLease)

func (g *mountGuard) run(mountName string, invalid bool, operation string, probe func() error) error {
	startProbe, generation, err := g.begin(mountName, invalid, operation)
	if err != nil {
		return err
	}
	if !startProbe {
		return nil
	}

	done := make(chan error, 1)
	go func() {
		done <- probe()
	}()

	select {
	case probeErr := <-done:
		g.finishProbe(mountName, generation, probeErr)
		return probeErr
	case <-time.After(g.probeTimeout):
		g.markProbeTimeout(mountName, generation)
		return fmt.Errorf("external mount %s probe timeout after %s (%s)", mountName, g.probeTimeout, operation)
	}
}

func (g *mountGuard) begin(mountName string, invalid bool, operation string) (bool, uint64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	state := g.states[mountName]
	if state == nil {
		state = &mountGuardState{}
		g.states[mountName] = state
	}

	now := time.Now()
	if !state.seenInvalid || state.lastInvalid != invalid {
		state.seenInvalid = true
		state.lastInvalid = invalid
		state.probing = false
		state.cooldownUntil = time.Time{}
		state.generation++
	}

	if invalid {
		return false, 0, fmt.Errorf("external mount %s unavailable: invalid=true (%s)", mountName, operation)
	}

	if state.probing && now.Sub(state.probeStarted) > g.probeLease {
		// Assume the previous probe is stuck in an uninterruptible syscall.
		// Release logical gate by rotating generation so late completion
		// from that probe is ignored.
		state.probing = false
		state.cooldownUntil = now.Add(g.cooldown)
		state.generation++
	}

	if state.probing {
		return false, 0, fmt.Errorf("external mount %s is being checked (%s)", mountName, operation)
	}

	if now.Before(state.cooldownUntil) {
		return false, 0, fmt.Errorf("external mount %s temporarily unavailable (%s)", mountName, operation)
	}

	state.probing = true
	state.probeStarted = now
	return true, state.generation, nil
}

func (g *mountGuard) finishProbe(mountName string, generation uint64, probeErr error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	state := g.states[mountName]
	if state == nil || state.generation != generation {
		return
	}

	state.probing = false
	state.probeStarted = time.Time{}
	if probeErr != nil {
		state.cooldownUntil = time.Now().Add(g.cooldown)
		return
	}
	state.cooldownUntil = time.Time{}
}

func (g *mountGuard) markProbeTimeout(mountName string, generation uint64) {
	g.mu.Lock()
	defer g.mu.Unlock()

	state := g.states[mountName]
	if state == nil || state.generation != generation {
		return
	}

	// Keep probing=true until probe goroutine returns so we only have
	// one in-flight probe per mount.
	state.cooldownUntil = time.Now().Add(g.cooldown)
}

func shouldGuardExternalMount(fileParam *models.FileParam) (string, bool) {
	if fileParam == nil || fileParam.FileType != common.External {
		return "", false
	}
	mountName, ok := getExternalMountName(fileParam.Path)
	if !ok {
		return "", false
	}
	return mountName, true
}

func runWithExternalMountGuard[T any](fileParam *models.FileParam, operation string, fn func() (T, error)) (T, error) {
	var zero T

	mountName, shouldGuard := shouldGuardExternalMount(fileParam)
	if !shouldGuard {
		return fn()
	}

	mounted, found := global.GlobalMounted.GetMountedByPath(mountName)
	if !found {
		// Only guard paths that are known mount roots in GlobalMounted.
		// Non-mounted local dirs under External should bypass guard.
		return fn()
	}

	var result T
	err := externalMountGuard.run(mountName, mounted.Invalid, operation, func() error {
		var innerErr error
		result, innerErr = fn()
		return innerErr
	})
	if err != nil {
		return zero, err
	}
	return result, nil
}
