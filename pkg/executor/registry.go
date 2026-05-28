package executor

import (
	"os/exec"
	"sync"
)

// ProcessTracker is a read-only view into the process registry.
// It is intended for consumers (such as a PID-1 zombie reaper) that need
// to check whether a PID is managed by the executor but must not modify
// the registry.
type ProcessTracker interface {
	// IsActive reports whether pid is currently tracked as a running process.
	IsActive(pid int) bool
}

// processRegistry tracks PIDs of processes started by the executor so that
// a PID-1 zombie reaper can skip them (their parent already calls Wait).
// This prevents the reaper from stealing a child that cmd.Wait expects to reap.
//
// The struct is intentionally unexported — all external access goes through
// the ProcessTracker interface (read-only) or the package-level helpers
// registerPID / unregisterPID (write, executor-internal).
type processRegistry struct {
	mu         sync.RWMutex
	activePIDs map[int]struct{}
}

// register adds pid to the set of active PIDs.
func (r *processRegistry) register(pid int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.activePIDs[pid] = struct{}{}
}

// unregister removes pid from the set of active PIDs.
func (r *processRegistry) unregister(pid int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.activePIDs, pid)
}

// startAndRegister calls cmd.Start() and, on success, registers the child
// PID under the same write-lock. This eliminates the window between process
// creation and registration during which a PID-1 zombie reaper could
// prematurely reap a fast-exiting child.
func (r *processRegistry) startAndRegister(cmd *exec.Cmd) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := cmd.Start(); err != nil {
		return err
	}

	r.activePIDs[cmd.Process.Pid] = struct{}{}

	return nil
}

// IsActive reports whether pid is currently tracked as an active process.
func (r *processRegistry) IsActive(pid int) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, ok := r.activePIDs[pid]

	return ok
}

// registry is the singleton process registry.
// It is not exported — external packages obtain a ProcessTracker via Tracker().
var registry = &processRegistry{
	activePIDs: make(map[int]struct{}),
}

// Tracker returns a read-only view of the global process registry.
// The zombie reaper should call this once and use the returned ProcessTracker
// to check whether a PID is managed by the executor.
func Tracker() ProcessTracker {
	return registry
}

// registerPID and unregisterPID are package-internal helpers used by Run,
// Output, and RunAndLogLines to track child PIDs.
func registerPID(pid int) {
	registry.register(pid)
}

func unregisterPID(pid int) {
	registry.unregister(pid)
}

// startAndRegister calls cmd.Start() and registers the resulting PID
// atomically under the registry's write-lock. Callers must still call
// unregisterPID(cmd.Process.Pid) (typically via defer) when cmd.Wait returns.
func startAndRegister(cmd *exec.Cmd) error {
	return registry.startAndRegister(cmd)
}
