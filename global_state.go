//go:build wails

package main

import (
	"database/sql"
	"strconv"
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
	globalDB               *sql.DB // riferimento al database per persistenza
)

// InitGlobalState inizializza lo stato globale e carica le impostazioni dal database
func InitGlobalState(db *sql.DB) {
	globalStateMu.Lock()
	defer globalStateMu.Unlock()

	globalIsTracking = false
	globalCurrentProject = nil
	globalWatcher = nil
	globalPendingSessionID = 0
	globalDB = db

	// Carica idle threshold dal database, usa 5 come default
	globalIdleThreshold = 5
	if db != nil {
		if value, err := tracker.GetSetting(db, "idle_threshold"); err == nil && value != "" {
			if threshold, err := strconv.Atoi(value); err == nil && threshold > 0 {
				globalIdleThreshold = threshold
			}
		}
	}
}

// SetGlobalIdleThreshold imposta la soglia di inattività e la salva nel database
func SetGlobalIdleThreshold(minutes int) {
	globalStateMu.Lock()
	defer globalStateMu.Unlock()
	globalIdleThreshold = minutes

	// Salva nel database per persistenza
	if globalDB != nil {
		tracker.SetSetting(globalDB, "idle_threshold", strconv.Itoa(minutes))
	}
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
