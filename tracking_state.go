package main

import (
	"strconv"
	"sync"

	"work-time-tracker-go/tracker"
)

// trackingState raccoglie lo stato del tracking in corso. È un campo di App, non una
// variabile globale: ogni istanza (e ogni test) ha il proprio stato e il proprio lock.
type trackingState struct {
	mu               sync.Mutex
	isTracking       bool
	currentProject   *tracker.Project
	watcher          *tracker.TimeWatcher
	pendingSessionID int64
	idleThreshold    int // soglia inattività in minuti
}

const defaultIdleThresholdMinutes = 5

// resetTrackingState azzera il tracking e ricarica la soglia di inattività dal database.
func (a *App) resetTrackingState() {
	a.track.mu.Lock()
	defer a.track.mu.Unlock()
	a.resetTrackingStateLocked()
}

// resetTrackingStateLocked richiede a.track.mu già acquisito.
func (a *App) resetTrackingStateLocked() {
	a.track.isTracking = false
	a.track.currentProject = nil
	a.track.watcher = nil
	a.track.pendingSessionID = 0
	a.track.idleThreshold = defaultIdleThresholdMinutes
	if a.db != nil {
		if value, err := tracker.GetSetting(a.db, "idle_threshold"); err == nil && value != "" {
			if threshold, err := strconv.Atoi(value); err == nil && threshold > 0 {
				a.track.idleThreshold = threshold
			}
		}
	}
}

// setIdleThresholdSetting salva la soglia nel database e la applica allo stato.
func (a *App) setIdleThresholdSetting(minutes int) error {
	a.track.mu.Lock()
	defer a.track.mu.Unlock()
	if a.db != nil {
		if err := tracker.SetSetting(a.db, "idle_threshold", strconv.Itoa(minutes)); err != nil {
			return err
		}
	}
	a.track.idleThreshold = minutes
	return nil
}
