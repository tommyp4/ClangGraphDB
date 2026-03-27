package progress

import (
	"sync"
)

var (
	activePbs   = make(map[any]bool)
	activePbsMu sync.Mutex
)

// IsAnyActive returns true if any progress bar or spinner is currently active.
func IsAnyActive() bool {
	activePbsMu.Lock()
	defer activePbsMu.Unlock()
	return len(activePbs) > 0
}

// RegisterActive registers a progress bar as active.
func RegisterActive(pb any) {
	activePbsMu.Lock()
	defer activePbsMu.Unlock()
	activePbs[pb] = true
}

// UnregisterActive unregisters a progress bar.
func UnregisterActive(pb any) {
	activePbsMu.Lock()
	defer activePbsMu.Unlock()
	delete(activePbs, pb)
}
