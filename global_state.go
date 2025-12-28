//go:build wails

package main

import (
	"sync"

	"work-time-tracker-go/tracker"
)

// Variabili globali per lo stato del tracking
var (
	globalIsTracking       bool
	globalCurrentProject   *tracker.Project
	globalWatcher          *tracker.TimeWatcher
	globalPendingSessionID int64
	globalIdleThreshold    int // soglia inattività in minuti
	globalStateMu          sync.Mutex
)

// InitGlobalState inizializza lo stato globale
func InitGlobalState() {
	globalStateMu.Lock()
	defer globalStateMu.Unlock()

	globalIsTracking = false
	globalCurrentProject = nil
	globalWatcher = nil
	globalPendingSessionID = 0
	globalIdleThreshold = 5 // default 5 minuti
}

// SetGlobalIdleThreshold imposta la soglia di inattività
func SetGlobalIdleThreshold(minutes int) {
	globalStateMu.Lock()
	defer globalStateMu.Unlock()
	globalIdleThreshold = minutes
}

// GetGlobalIdleThreshold ritorna la soglia di inattività
func GetGlobalIdleThreshold() int {
	globalStateMu.Lock()
	defer globalStateMu.Unlock()
	return globalIdleThreshold
}

// SetGlobalTrackingState aggiorna lo stato globale del tracking
func SetGlobalTrackingState(watcher *tracker.TimeWatcher, project *tracker.Project, running bool, pendingSessionID int64) {
	globalStateMu.Lock()
	defer globalStateMu.Unlock()

	globalWatcher = watcher
	globalCurrentProject = project
	globalIsTracking = running
	globalPendingSessionID = pendingSessionID
}

// GetGlobalTrackingStateWails ritorna lo stato corrente del tracking per Wails
func GetGlobalTrackingStateWails() (watcher *tracker.TimeWatcher, project *tracker.Project, running bool, sessionID int64) {
	globalStateMu.Lock()
	defer globalStateMu.Unlock()
	return globalWatcher, globalCurrentProject, globalIsTracking, globalPendingSessionID
}
