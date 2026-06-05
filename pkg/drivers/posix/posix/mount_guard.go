package posix

import (
	"errors"
	"files/pkg/common"
	"files/pkg/files"
	"files/pkg/global"
	"files/pkg/models"
	"os"
	"path/filepath"
	"sync"
	"time"

	"k8s.io/klog/v2"
)

const (
	defaultMountProbeTimeout  = 8 * time.Second
	defaultMountCooldown      = 45 * time.Second
	defaultMountProbeLease    = 30 * time.Second
	defaultMountProbeInterval = 5 * time.Second
	defaultMountHealthyGrace  = 5 * time.Second
)

type mountProbeState string

const (
	mountProbeUnknown   mountProbeState = "unknown"
	mountProbeProbing   mountProbeState = "probing"
	mountProbeHealthy   mountProbeState = "healthy"
	mountProbeUnhealthy mountProbeState = "unhealthy"
)

type mountGuardState struct {
	mounted         bool
	reportedInvalid bool
	probeState      mountProbeState
	probeStarted    time.Time
	cooldownUntil   time.Time
	lastHealthyAt   time.Time
	lastProbeAt     time.Time
	lastProbeErr    string
	generation      uint64
	probePath       string
	stopCh          chan struct{}
}

type mountGuard struct {
	mu            sync.Mutex
	probeTimeout  time.Duration
	cooldown      time.Duration
	probeLease    time.Duration
	probeInterval time.Duration
	healthyGrace  time.Duration
	states        map[string]*mountGuardState
	probe         func(string) error
}

func newMountGuard(probeTimeout, cooldown, probeLease time.Duration) *mountGuard {
	return &mountGuard{
		probeTimeout:  probeTimeout,
		cooldown:      cooldown,
		probeLease:    probeLease,
		probeInterval: defaultMountProbeInterval,
		healthyGrace:  defaultMountHealthyGrace,
		states:        make(map[string]*mountGuardState),
		probe: func(path string) error {
			_, err := os.Lstat(path)
			return err
		},
	}
}

var externalMountGuard = newMountGuard(defaultMountProbeTimeout, defaultMountCooldown, defaultMountProbeLease)

func init() {
	global.RegisterMountedChangeListener(func(disks []files.DiskInfo) {
		externalMountGuard.reconcile(disks)
	})
}

func externalMountUnavailableError(mountName, operation, reason string) error {
	klog.Warningf("external mount unavailable, mount: %s, operation: %s, reason: %s", mountName, operation, reason)
	return errors.New(common.ErrorMessageExternalMountUnavailable)
}

func mountProbePath(mountName string) string {
	return filepath.Join(common.EXTERNAL_PREFIX, mountName)
}

func (g *mountGuard) reconcile(disks []files.DiskInfo) {
	type probeLoopStart struct {
		mountName  string
		generation uint64
		stopCh     <-chan struct{}
	}

	var starts []probeLoopStart
	seen := make(map[string]struct{}, len(disks))

	g.mu.Lock()
	for _, disk := range disks {
		if disk.Path == "" {
			continue
		}
		seen[disk.Path] = struct{}{}
		if start := g.observeMountLocked(disk.Path, &disk); start != nil {
			starts = append(starts, *start)
		}
	}

	for mountName, state := range g.states {
		if _, ok := seen[mountName]; ok {
			continue
		}
		g.stopProbeLoopLocked(state)
		delete(g.states, mountName)
	}
	g.mu.Unlock()

	for _, start := range starts {
		go g.runProbeLoop(start.mountName, start.generation, start.stopCh)
	}
}

func (g *mountGuard) observeMount(mountName string, mounted *files.DiskInfo) {
	if mounted == nil {
		return
	}

	var start *struct {
		mountName  string
		generation uint64
		stopCh     <-chan struct{}
	}

	g.mu.Lock()
	if s := g.observeMountLocked(mountName, mounted); s != nil {
		start = &struct {
			mountName  string
			generation uint64
			stopCh     <-chan struct{}
		}{
			mountName:  s.mountName,
			generation: s.generation,
			stopCh:     s.stopCh,
		}
	}
	g.mu.Unlock()

	if start != nil {
		go g.runProbeLoop(start.mountName, start.generation, start.stopCh)
	}
}

func (g *mountGuard) observeMountLocked(mountName string, mounted *files.DiskInfo) *struct {
	mountName  string
	generation uint64
	stopCh     <-chan struct{}
} {
	state := g.states[mountName]
	if state == nil {
		state = &mountGuardState{
			mounted:    true,
			probeState: mountProbeUnknown,
			probePath:  mountProbePath(mountName),
		}
		g.states[mountName] = state
	}

	state.mounted = true
	state.reportedInvalid = mounted.Invalid
	if state.probePath == "" {
		state.probePath = mountProbePath(mountName)
	}
	if state.stopCh != nil {
		return nil
	}

	state.generation++
	state.stopCh = make(chan struct{})
	return &struct {
		mountName  string
		generation uint64
		stopCh     <-chan struct{}
	}{
		mountName:  mountName,
		generation: state.generation,
		stopCh:     state.stopCh,
	}
}

func (g *mountGuard) stopProbeLoopLocked(state *mountGuardState) {
	if state == nil {
		return
	}
	if state.stopCh != nil {
		close(state.stopCh)
		state.stopCh = nil
	}
	state.mounted = false
	state.generation++
}

func (g *mountGuard) ensureAvailable(mountName string, mounted *files.DiskInfo, operation string) error {
	g.observeMount(mountName, mounted)

	now := time.Now()
	g.mu.Lock()
	state := g.states[mountName]
	if state == nil || !state.mounted {
		g.mu.Unlock()
		return externalMountUnavailableError(mountName, operation, "unmounted")
	}

	if state.reportedInvalid {
		g.mu.Unlock()
		return externalMountUnavailableError(mountName, operation, "invalid=true")
	}

	if state.probeState == mountProbeHealthy {
		g.mu.Unlock()
		return nil
	}

	if state.probeState == mountProbeProbing && !state.lastHealthyAt.IsZero() && now.Sub(state.lastHealthyAt) <= g.healthyGrace {
		g.mu.Unlock()
		return nil
	}

	if !state.cooldownUntil.IsZero() && now.Before(state.cooldownUntil) {
		g.mu.Unlock()
		return externalMountUnavailableError(mountName, operation, "cooldown active")
	}

	reason := "probe not healthy"
	if state.probeState == mountProbeProbing {
		reason = "probe in progress"
	} else if state.probeState == mountProbeUnknown {
		reason = "probe pending"
	} else if state.lastProbeErr != "" {
		reason = state.lastProbeErr
	}
	g.mu.Unlock()

	return externalMountUnavailableError(mountName, operation, reason)
}

func (g *mountGuard) runProbeLoop(mountName string, generation uint64, stopCh <-chan struct{}) {
	delay := time.Duration(0)
	for {
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-timer.C:
			case <-stopCh:
				timer.Stop()
				return
			}
		}

		probePath, ok := g.beginProbe(mountName, generation)
		if !ok {
			return
		}

		done := make(chan error, 1)
		go func() {
			done <- g.probe(probePath)
		}()

		select {
		case probeErr := <-done:
			delay = g.finishProbe(mountName, generation, probeErr)
		case <-time.After(g.probeTimeout):
			delay = g.markProbeTimeout(mountName, generation)
			select {
			case <-done:
				g.finishTimedOutProbe(mountName, generation)
			case <-stopCh:
				return
			}
		case <-stopCh:
			return
		}
	}
}

func (g *mountGuard) beginProbe(mountName string, generation uint64) (string, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	state := g.states[mountName]
	if state == nil || state.generation != generation || !state.mounted {
		return "", false
	}

	now := time.Now()
	if state.probeState == mountProbeProbing && !state.probeStarted.IsZero() && now.Sub(state.probeStarted) <= g.probeLease {
		return "", false
	}

	state.probeState = mountProbeProbing
	state.probeStarted = now
	state.lastProbeErr = ""
	if state.probePath == "" {
		state.probePath = mountProbePath(mountName)
	}
	return state.probePath, true
}

func (g *mountGuard) finishProbe(mountName string, generation uint64, probeErr error) time.Duration {
	g.mu.Lock()
	defer g.mu.Unlock()

	state := g.states[mountName]
	if state == nil || state.generation != generation || !state.mounted {
		return g.probeInterval
	}

	now := time.Now()
	state.probeStarted = time.Time{}
	state.lastProbeAt = now
	if probeErr != nil {
		state.probeState = mountProbeUnhealthy
		state.cooldownUntil = now.Add(g.cooldown)
		state.lastProbeErr = probeErr.Error()
		return g.cooldown
	}

	state.probeState = mountProbeHealthy
	state.cooldownUntil = time.Time{}
	state.lastProbeErr = ""
	state.lastHealthyAt = now
	return g.probeInterval
}

func (g *mountGuard) markProbeTimeout(mountName string, generation uint64) time.Duration {
	g.mu.Lock()
	defer g.mu.Unlock()

	state := g.states[mountName]
	if state == nil || state.generation != generation || !state.mounted {
		return g.probeInterval
	}

	now := time.Now()
	state.probeState = mountProbeUnhealthy
	state.lastProbeAt = now
	state.lastProbeErr = "probe timeout"
	state.cooldownUntil = now.Add(g.cooldown)
	return g.cooldown
}

func (g *mountGuard) finishTimedOutProbe(mountName string, generation uint64) {
	g.mu.Lock()
	defer g.mu.Unlock()

	state := g.states[mountName]
	if state == nil || state.generation != generation || !state.mounted {
		return
	}
	state.probeStarted = time.Time{}
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
	if err := externalMountGuard.ensureAvailable(mountName, mounted, operation); err != nil {
		return zero, err
	}
	result, err := fn()
	if err != nil {
		return zero, err
	}
	return result, nil
}
