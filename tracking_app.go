package main

import (
	"fmt"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"work-time-tracker-go/tracker"
)

// TrackingState rappresenta lo stato del tracking
type TrackingState struct {
	IsTracking     bool    `json:"is_tracking"`
	ProjectID      *int    `json:"project_id,omitempty"`
	ProjectName    string  `json:"project_name,omitempty"`
	ActivityType   *string `json:"activity_type,omitempty"`
	StartTime      string  `json:"start_time,omitempty"`
	ElapsedSeconds int     `json:"elapsed_seconds"`
	SessionID      int64   `json:"session_id,omitempty"`
}

// GetTrackingState restituisce lo stato corrente del tracking
func (a *App) GetTrackingState() TrackingState {
	a.track.mu.Lock()
	defer a.track.mu.Unlock()

	state := TrackingState{
		IsTracking: a.track.isTracking,
		SessionID:  a.track.pendingSessionID,
	}

	if a.track.isTracking && a.track.currentProject != nil {
		state.ProjectID = &a.track.currentProject.ID
		state.ProjectName = a.track.currentProject.Name
	}

	if a.track.watcher != nil {
		state.ElapsedSeconds = a.track.watcher.GetTotalActiveSeconds()
	}

	return state
}

// trackingCheckpointSeconds limita la perdita di tempo in caso di crash: al riavvio
// si recupera fino all'ultimo checkpoint (il watcher conta a passi di 5 secondi).
const trackingCheckpointSeconds = 30

// StartTracking avvia il tracking per un progetto
func (a *App) StartTracking(projectID int, activityType *string) error {
	if projectID <= 0 {
		return fmt.Errorf("ID progetto non valido")
	}
	if err := a.validateNewActivityReference(activityType); err != nil {
		return err
	}
	// Mantieni il lock per l'intera inizializzazione: due richieste quasi
	// simultanee non possono creare watcher o sessioni pendenti duplicate.
	a.track.mu.Lock()
	defer a.track.mu.Unlock()
	if a.track.isTracking {
		return fmt.Errorf("un tracking è già in corso")
	}
	if a.track.watcher != nil {
		return fmt.Errorf("attribuisci prima il periodo di inattività pendente")
	}
	project, err := tracker.TrovaProgettoById(a.db, projectID)
	if err != nil {
		return err
	}
	if project.Archived {
		return fmt.Errorf("non è possibile tracciare un progetto archiviato")
	}

	// Crea sessione pendente
	startTime := time.Now().Format("2006-01-02 15:04:05")
	sessionID, err := tracker.StartPendingTracking(a.db, &projectID, activityType, startTime)
	if err != nil {
		return err
	}

	// Crea watcher
	watcher := tracker.NewTimeWatcher()
	idleMinutes := a.track.idleThreshold
	watcher.SetIdleThreshold(idleMinutes * 60) // Converti minuti in secondi

	// Imposta callback per salvataggio periodico
	watcher.SetSaveCallback(func(totalSeconds int) error {
		return tracker.UpdatePendingTracking(a.db, sessionID, totalSeconds)
	}, trackingCheckpointSeconds)

	// Imposta callback per auto-stop quando viene rilevato idle
	watcher.SetOnIdleCallback(func() {
		a.autoStopOnIdle(watcher, sessionID)
	})

	// Imposta callback per notifica quando l'utente torna dall'idle
	watcher.SetOnIdleReturnCallback(func(minutes int) {
		runtime.EventsEmit(a.ctx, "idle-returned", map[string]interface{}{"minutes": minutes})
		// Invia notifica toast Windows
		a.ShowIdleNotification(minutes)
		// Prova anche a portare la finestra in primo piano
		a.BringWindowToFront()
	})

	watcher.Start(5)

	// Aggiorna lo stato mentre il lock è ancora detenuto.
	a.track.watcher = watcher
	a.track.currentProject = project
	a.track.isTracking = true
	a.track.pendingSessionID = sessionID

	return nil
}

// autoStopOnIdle ferma automaticamente il tracking quando viene rilevato idle
func (a *App) autoStopOnIdle(watcher *tracker.TimeWatcher, sessionID int64) {
	a.track.mu.Lock()
	// Verifica che il tracking sia ancora attivo con questo watcher
	if !a.track.isTracking || a.track.watcher != watcher {
		a.track.mu.Unlock()
		fmt.Println("[IDLE-AUTOSTOP] Tracking già fermato o watcher diverso, skip")
		return
	}

	// Rivendica atomicamente lo stop. Il watcher resta associato allo stato
	// globale soltanto per consentire l'attribuzione del periodo idle.
	a.track.isTracking = false
	a.track.currentProject = nil
	a.track.mu.Unlock()

	watcher.PauseCounting()

	// Ferma il watcher (non blocca perché chiamato dalla goroutine del watcher stesso)
	// NON chiamare watcher.Stop() qui perché siamo già dentro la goroutine
	finalSeconds := watcher.GetTotalActiveSeconds()

	// Salva la sessione finale nel database
	if sessionID > 0 {
		err := tracker.FinalizePendingTracking(a.db, sessionID, finalSeconds)
		if err != nil {
			fmt.Printf("[IDLE-AUTOSTOP] Errore salvataggio sessione: %v\n", err)
		} else {
			a.track.mu.Lock()
			if a.track.watcher == watcher && a.track.pendingSessionID == sessionID {
				a.track.pendingSessionID = 0
			}
			a.track.mu.Unlock()
			fmt.Printf("[IDLE-AUTOSTOP] Sessione salvata: %d secondi (%d min)\n", finalSeconds, finalSeconds/60)
		}
	}

	// Emetti evento al frontend per notificare l'auto-stop
	runtime.EventsEmit(a.ctx, "tracking-auto-stopped", map[string]interface{}{
		"seconds": finalSeconds,
		"reason":  "idle",
	})

	fmt.Println("[IDLE-AUTOSTOP] Tracking fermato automaticamente per inattività")
}

// StopTracking ferma il tracking corrente
func (a *App) StopTracking() (int, error) {
	a.track.mu.Lock()
	if !a.track.isTracking || a.track.watcher == nil {
		a.track.mu.Unlock()
		return 0, fmt.Errorf("nessun tracking in corso")
	}

	watcher := a.track.watcher
	sessionID := a.track.pendingSessionID
	a.track.mu.Unlock()

	// Lo stato globale resta "in tracking" finché il salvataggio non riesce: se la
	// finalizzazione fallisce l'utente può riprovare e la sessione pendente resta
	// recuperabile, invece di sembrare conclusa.
	watcher.Stop()
	finalSeconds := watcher.GetTotalActiveSeconds()

	if sessionID > 0 {
		if err := tracker.FinalizePendingTracking(a.db, sessionID, finalSeconds); err != nil {
			a.track.mu.Lock()
			alreadyStopped := !a.track.isTracking || a.track.watcher != watcher
			a.track.mu.Unlock()
			if alreadyStopped {
				return finalSeconds, nil // fermato nel frattempo da auto-stop o da un doppio invio
			}
			return 0, fmt.Errorf("salvataggio della sessione non riuscito, riprova a fermare il tracking: %w", err)
		}
	}

	a.track.mu.Lock()
	if a.track.watcher == watcher {
		a.track.isTracking = false
		a.track.currentProject = nil
		a.track.watcher = nil
		a.track.pendingSessionID = 0
	}
	a.track.mu.Unlock()

	return finalSeconds, nil
}

// IdlePeriodData rappresenta un periodo di inattività pendente
type IdlePeriodData struct {
	HasPending bool   `json:"has_pending"`
	Minutes    int    `json:"minutes"`
	StartTime  string `json:"start_time"`
	EndTime    string `json:"end_time"`
}

// CheckIdlePeriod verifica se c'è un periodo idle pendente
func (a *App) CheckIdlePeriod() IdlePeriodData {
	a.track.mu.Lock()
	defer a.track.mu.Unlock()

	if a.track.watcher == nil {
		return IdlePeriodData{HasPending: false}
	}

	idlePeriod := a.track.watcher.GetPendingIdlePeriod()
	if idlePeriod == nil {
		return IdlePeriodData{HasPending: false}
	}

	return IdlePeriodData{
		HasPending: true,
		Minutes:    idlePeriod.Duration / 60, // Duration is in seconds
		StartTime:  idlePeriod.StartTime.Format("15:04"),
		EndTime:    idlePeriod.EndTime.Format("15:04"),
	}
}

// AttributeIdle attribuisce il tempo idle a un progetto o come pausa
func (a *App) AttributeIdle(projectID int, isBreak bool) error {
	a.track.mu.Lock()
	defer a.track.mu.Unlock()

	if a.track.watcher == nil {
		return fmt.Errorf("nessun watcher disponibile")
	}

	idlePeriod := a.track.watcher.GetPendingIdlePeriod()
	if idlePeriod == nil {
		return fmt.Errorf("nessun periodo idle pendente")
	}
	if !a.track.isTracking && a.track.pendingSessionID > 0 {
		if err := tracker.FinalizePendingTracking(a.db, a.track.pendingSessionID, a.track.watcher.GetTotalActiveSeconds()); err != nil {
			return fmt.Errorf("impossibile completare il salvataggio del tracking: %w", err)
		}
		a.track.pendingSessionID = 0
	}

	if isBreak {
		// Registra come pausa (non viene salvato come sessione)
		a.track.watcher.ClearPendingIdlePeriod()
		// Pulizia: se il tracking era già stato fermato (auto-stop), ferma il watcher
		if !a.track.isTracking {
			a.track.watcher.Stop()
			a.track.watcher = nil
		}
		return nil
	}

	// Attribuisci al progetto
	if projectID <= 0 {
		return fmt.Errorf("ID progetto non valido")
	}
	project, err := tracker.TrovaProgettoById(a.db, projectID)
	if err != nil {
		return err
	}
	if project.Archived {
		return fmt.Errorf("non è possibile attribuire l'inattività a un progetto archiviato")
	}

	// Crea una sessione per il tempo idle
	seconds := idlePeriod.Duration // Duration è già in secondi
	timestamp := idlePeriod.StartTime.Format("2006-01-02 15:04:05")
	err = tracker.CreaSessione(a.db, "Tempo Idle", seconds, &projectID, "off-computer", nil, timestamp)
	if err != nil {
		return err
	}

	a.track.watcher.ClearPendingIdlePeriod()
	// Pulizia: se il tracking era già stato fermato (auto-stop), ferma il watcher
	if !a.track.isTracking {
		a.track.watcher.Stop()
		a.track.watcher = nil
	}
	return nil
}

// SetIdleThreshold imposta la soglia di inattività in minuti
func (a *App) SetIdleThreshold(minutes int) error {
	if minutes < 1 || minutes > 1440 {
		return fmt.Errorf("la soglia di inattività deve essere compresa tra 1 e 1440 minuti")
	}
	if err := a.setIdleThresholdSetting(minutes); err != nil {
		return err
	}

	a.track.mu.Lock()
	watcher := a.track.watcher
	a.track.mu.Unlock()
	if watcher != nil {
		watcher.SetIdleThreshold(minutes * 60)
	}
	return nil
}

// GetIdleThreshold ritorna la soglia di inattività corrente in minuti
func (a *App) GetIdleThreshold() int {
	a.track.mu.Lock()
	defer a.track.mu.Unlock()
	return a.track.idleThreshold
}
