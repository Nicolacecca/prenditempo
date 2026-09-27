package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
	"work-time-tracker-go/tracker"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct per Wails - espone metodi al frontend
type App struct {
	ctx context.Context
	db  *sql.DB
}

// NewApp crea una nuova istanza App
func NewApp() *App {
	return &App{}
}

// startup viene chiamato all'avvio dell'app
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// SetDB imposta il database
func (a *App) SetDB(db *sql.DB) {
	a.db = db
}

// === PROGETTI ===

// ProjectData rappresenta i dati di un progetto
type ProjectData struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	Archived    bool   `json:"archived"`
	ClosedAt    string `json:"closed_at,omitempty"`
	NoteText    string `json:"note_text"`
}

// GetProjects restituisce tutti i progetti attivi (ottimizzato con query filtrata)
func (a *App) GetProjects() ([]ProjectData, error) {
	projects, err := tracker.CaricaProgettiAttivi(a.db)
	if err != nil {
		return nil, err
	}

	var result []ProjectData
	for _, p := range projects {
		result = append(result, ProjectData{
			ID:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			CreatedAt:   p.CreatedAt,
			Archived:    p.Archived,
			ClosedAt:    p.ClosedAt,
			NoteText:    p.NoteText,
		})
	}
	return result, nil
}

// GetArchivedProjects restituisce i progetti archiviati (ottimizzato con query filtrata)
func (a *App) GetArchivedProjects() ([]ProjectData, error) {
	projects, err := tracker.CaricaProgettiArchiviati(a.db)
	if err != nil {
		return nil, err
	}

	var result []ProjectData
	for _, p := range projects {
		result = append(result, ProjectData{
			ID:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			CreatedAt:   p.CreatedAt,
			Archived:    p.Archived,
			ClosedAt:    p.ClosedAt,
			NoteText:    p.NoteText,
		})
	}
	return result, nil
}

// CreateProject crea un nuovo progetto
func (a *App) CreateProject(name, description string) (int64, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if name == "" || len([]rune(name)) > 100 {
		return 0, fmt.Errorf("il nome del progetto è obbligatorio e non può superare 100 caratteri")
	}
	if len([]rune(description)) > 500 {
		return 0, fmt.Errorf("la descrizione non può superare 500 caratteri")
	}
	return tracker.CreaProgetto(a.db, name, description)
}

// ArchiveProject archivia un progetto
func (a *App) ArchiveProject(projectID int) error {
	if projectID <= 0 {
		return fmt.Errorf("ID progetto non valido")
	}
	globalStateMu.Lock()
	defer globalStateMu.Unlock()
	if err := ensureProjectNotTrackedLocked(projectID); err != nil {
		return err
	}
	return tracker.ArchivaProgetto(a.db, projectID)
}

// ReactivateProject riattiva un progetto archiviato
func (a *App) ReactivateProject(projectID int) error {
	if projectID <= 0 {
		return fmt.Errorf("ID progetto non valido")
	}
	return tracker.RiattivaProgetto(a.db, projectID)
}

// DeleteProject elimina un progetto
func (a *App) DeleteProject(projectID int) error {
	if projectID <= 0 {
		return fmt.Errorf("ID progetto non valido")
	}
	globalStateMu.Lock()
	defer globalStateMu.Unlock()
	if err := ensureProjectNotTrackedLocked(projectID); err != nil {
		return err
	}
	return tracker.EliminaProgettoById(a.db, projectID)
}

func ensureProjectNotTrackedLocked(projectID int) error {
	if globalIsTracking && globalCurrentProject != nil && globalCurrentProject.ID == projectID {
		return fmt.Errorf("il progetto è in uso dal tracking corrente")
	}
	return nil
}

// UpdateProject aggiorna nome e descrizione di un progetto
func (a *App) UpdateProject(projectID int, name, description string) error {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if projectID <= 0 || name == "" || len([]rune(name)) > 100 {
		return fmt.Errorf("dati progetto non validi")
	}
	if len([]rune(description)) > 500 {
		return fmt.Errorf("la descrizione non può superare 500 caratteri")
	}
	return tracker.AggiornaProgetto(a.db, projectID, name, description)
}

// UpdateProjectNote aggiorna la nota markdown di un progetto
func (a *App) UpdateProjectNote(projectID int, noteText string) error {
	if projectID <= 0 || len([]byte(noteText)) > 1024*1024 {
		return fmt.Errorf("nota progetto non valida o superiore a 1 MiB")
	}
	return tracker.AggiornaNotaProgetto(a.db, projectID, noteText)
}

// MigrateLegacyNotes migra le note dalla vecchia tabella al nuovo sistema
func (a *App) MigrateLegacyNotes() (int, error) {
	return tracker.MigraNoteLegacy(a.db)
}

// GetProjectReport genera il report di un progetto
func (a *App) GetProjectReport(projectID int) (map[string]interface{}, error) {
	return tracker.GeneraReportChiusura(a.db, projectID)
}

// === SESSIONI ===

// SessionData rappresenta una sessione
type SessionData struct {
	ID           int     `json:"id"`
	AppName      string  `json:"app_name"`
	Seconds      int     `json:"seconds"`
	ProjectID    *int    `json:"project_id,omitempty"`
	ProjectName  string  `json:"project_name"`
	SessionType  string  `json:"session_type"`
	ActivityType *string `json:"activity_type,omitempty"`
	Timestamp    string  `json:"timestamp"`
}

// GetSessions restituisce le sessioni in un periodo
func (a *App) GetSessions(startDate, endDate string) ([]SessionData, error) {
	sessions, err := tracker.CaricaSessioniDettagliate(a.db, startDate, endDate)
	if err != nil {
		return nil, err
	}

	var result []SessionData
	for _, s := range sessions {
		result = append(result, SessionData{
			ID:           s.ID,
			AppName:      s.AppName,
			Seconds:      s.Seconds,
			ProjectID:    s.ProjectID,
			ProjectName:  s.ProjectName,
			SessionType:  s.SessionType,
			ActivityType: s.ActivityType,
			Timestamp:    s.Timestamp,
		})
	}
	return result, nil
}

// CreateSession crea una nuova sessione
func (a *App) CreateSession(appName string, seconds int, projectID *int, sessionType string, activityType *string, timestamp string) error {
	appName = strings.TrimSpace(appName)
	if appName == "" || len([]rune(appName)) > 200 {
		return fmt.Errorf("nome sessione non valido")
	}
	if seconds <= 0 || seconds > 366*24*60*60 {
		return fmt.Errorf("durata sessione non valida")
	}
	if sessionType != "computer" && sessionType != "off-computer" && sessionType != "manual" {
		return fmt.Errorf("tipo sessione non valido")
	}
	if _, err := parseLocalTimestamp(timestamp); err != nil {
		return err
	}
	if err := a.validateActivityReference(activityType); err != nil {
		return err
	}
	if projectID != nil {
		project, err := tracker.TrovaProgettoById(a.db, *projectID)
		if err != nil {
			return err
		}
		if project.Archived {
			return fmt.Errorf("non è possibile aggiungere una sessione a un progetto archiviato")
		}
	}
	return tracker.CreaSessione(a.db, appName, seconds, projectID, sessionType, activityType, timestamp)
}

// UpdateSessionDuration aggiorna la durata di una sessione
func (a *App) UpdateSessionDuration(sessionID, seconds int) error {
	if sessionID <= 0 || seconds <= 0 || seconds > 366*24*60*60 {
		return fmt.Errorf("sessione o durata non valida")
	}
	return tracker.AggiornaDurataSessione(a.db, sessionID, seconds)
}

// UpdateSessionActivityType aggiorna il tipo di attività di una sessione
func (a *App) UpdateSessionActivityType(sessionID int, activityType *string) error {
	if sessionID <= 0 {
		return fmt.Errorf("ID sessione non valido")
	}
	if err := a.validateActivityReference(activityType); err != nil {
		return err
	}
	return tracker.AggiornaActivityType(a.db, sessionID, activityType)
}

// DeleteSession elimina una sessione
func (a *App) DeleteSession(sessionID int) error {
	if sessionID <= 0 {
		return fmt.Errorf("ID sessione non valido")
	}
	return tracker.EliminaSessione(a.db, sessionID)
}

// SplitSession divide una sessione in due parti
func (a *App) SplitSession(sessionID, firstPartSeconds int, firstActivityType, secondActivityType *string) error {
	if sessionID <= 0 || firstPartSeconds <= 0 {
		return fmt.Errorf("dati divisione non validi")
	}
	if err := a.validateActivityReference(firstActivityType); err != nil {
		return err
	}
	if err := a.validateActivityReference(secondActivityType); err != nil {
		return err
	}
	return tracker.DividiSessione(a.db, sessionID, firstPartSeconds, firstActivityType, secondActivityType)
}

// UpdateSessionComplete aggiorna timestamp, durata e tipo attività di una sessione
func (a *App) UpdateSessionComplete(sessionID int, newTimestamp string, newSeconds int, activityType *string) error {
	if sessionID <= 0 || newSeconds <= 0 || newSeconds > 366*24*60*60 {
		return fmt.Errorf("sessione o durata non valida")
	}
	if _, err := parseLocalTimestamp(newTimestamp); err != nil {
		return err
	}
	if err := a.validateActivityReference(activityType); err != nil {
		return err
	}
	return tracker.AggiornaSessioneCompleta(a.db, sessionID, newTimestamp, newSeconds, activityType)
}

// GetSessionById restituisce una sessione dato l'ID
func (a *App) GetSessionById(sessionID int) (*SessionData, error) {
	session, err := tracker.CaricaSessioneById(a.db, sessionID)
	if err != nil {
		return nil, err
	}

	return &SessionData{
		ID:           session.ID,
		AppName:      session.AppName,
		Seconds:      session.Seconds,
		ProjectID:    session.ProjectID,
		ProjectName:  session.ProjectName,
		SessionType:  session.SessionType,
		ActivityType: session.ActivityType,
		Timestamp:    session.Timestamp,
	}, nil
}

// === TIPI DI ATTIVITÀ ===

// ActivityTypeData rappresenta un tipo di attività
type ActivityTypeData struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	ColorVariant float64 `json:"color_variant"`
	Pattern      string  `json:"pattern"`
	DisplayOrder int     `json:"display_order"`
}

// GetActivityTypes restituisce tutti i tipi di attività
func (a *App) GetActivityTypes() ([]ActivityTypeData, error) {
	types, err := tracker.CaricaTipiAttivita(a.db)
	if err != nil {
		return nil, err
	}

	var result []ActivityTypeData
	for _, t := range types {
		result = append(result, ActivityTypeData{
			ID:           t.ID,
			Name:         t.Name,
			ColorVariant: t.ColorVariant,
			Pattern:      t.Pattern,
			DisplayOrder: t.DisplayOrder,
		})
	}
	return result, nil
}

// CreateActivityType crea un nuovo tipo di attività
func (a *App) CreateActivityType(name string, colorVariant float64, pattern string, displayOrder int) (int64, error) {
	if err := validateActivityType(name, colorVariant, pattern, displayOrder); err != nil {
		return 0, err
	}
	return tracker.CreaTipoAttivita(a.db, name, colorVariant, pattern, displayOrder)
}

// UpdateActivityType aggiorna un tipo di attività
func (a *App) UpdateActivityType(id int, name string, colorVariant float64, pattern string, displayOrder int) error {
	if id <= 0 {
		return fmt.Errorf("ID tipo attività non valido")
	}
	if err := validateActivityType(name, colorVariant, pattern, displayOrder); err != nil {
		return err
	}
	return tracker.AggiornaTipoAttivita(a.db, id, name, colorVariant, pattern, displayOrder)
}

// DeleteActivityType elimina un tipo di attività
func (a *App) DeleteActivityType(id int) error {
	if id <= 0 {
		return fmt.Errorf("ID tipo attività non valido")
	}
	return tracker.EliminaTipoAttivita(a.db, id)
}

// ReorderActivityTypes aggiorna l'ordine dei tipi di attività
func (a *App) ReorderActivityTypes(updates []map[string]int) error {
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	seen := make(map[int]struct{}, len(updates))
	for _, u := range updates {
		id, idOK := u["id"]
		order, orderOK := u["display_order"]
		if !idOK || !orderOK || id <= 0 || order < 0 {
			return fmt.Errorf("riordinamento attività non valido")
		}
		if _, duplicate := seen[id]; duplicate {
			return fmt.Errorf("tipo attività duplicato nel riordinamento")
		}
		seen[id] = struct{}{}
		result, err := tx.Exec("UPDATE activity_types SET display_order = ? WHERE id = ?", order, id)
		if err != nil {
			return err
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return fmt.Errorf("tipo attività %d non trovato", id)
		}
	}
	return tx.Commit()
}

// === TRACKING ===

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
	globalStateMu.Lock()
	defer globalStateMu.Unlock()

	state := TrackingState{
		IsTracking: globalIsTracking,
		SessionID:  globalPendingSessionID,
	}

	if globalIsTracking && globalCurrentProject != nil {
		state.ProjectID = &globalCurrentProject.ID
		state.ProjectName = globalCurrentProject.Name
	}

	if globalWatcher != nil {
		state.ElapsedSeconds = globalWatcher.GetTotalActiveSeconds()
	}

	return state
}

// StartTracking avvia il tracking per un progetto
func (a *App) StartTracking(projectID int, activityType *string) error {
	if projectID <= 0 {
		return fmt.Errorf("ID progetto non valido")
	}
	if err := a.validateActivityReference(activityType); err != nil {
		return err
	}
	// Mantieni il lock per l'intera inizializzazione: due richieste quasi
	// simultanee non possono creare watcher o sessioni pendenti duplicate.
	globalStateMu.Lock()
	defer globalStateMu.Unlock()
	if globalIsTracking {
		return fmt.Errorf("un tracking è già in corso")
	}
	if globalWatcher != nil {
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
	idleMinutes := globalIdleThreshold
	watcher.SetIdleThreshold(idleMinutes * 60) // Converti minuti in secondi

	// Imposta callback per salvataggio periodico
	watcher.SetSaveCallback(func(totalSeconds int) error {
		return tracker.UpdatePendingTracking(a.db, sessionID, totalSeconds)
	}, 300) // Ogni 5 minuti

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
	globalWatcher = watcher
	globalCurrentProject = project
	globalIsTracking = true
	globalPendingSessionID = sessionID

	return nil
}

// autoStopOnIdle ferma automaticamente il tracking quando viene rilevato idle
func (a *App) autoStopOnIdle(watcher *tracker.TimeWatcher, sessionID int64) {
	globalStateMu.Lock()
	// Verifica che il tracking sia ancora attivo con questo watcher
	if !globalIsTracking || globalWatcher != watcher {
		globalStateMu.Unlock()
		fmt.Println("[IDLE-AUTOSTOP] Tracking già fermato o watcher diverso, skip")
		return
	}

	// Rivendica atomicamente lo stop. Il watcher resta associato allo stato
	// globale soltanto per consentire l'attribuzione del periodo idle.
	globalIsTracking = false
	globalCurrentProject = nil
	globalStateMu.Unlock()

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
			globalStateMu.Lock()
			if globalWatcher == watcher && globalPendingSessionID == sessionID {
				globalPendingSessionID = 0
			}
			globalStateMu.Unlock()
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
	globalStateMu.Lock()
	if !globalIsTracking || globalWatcher == nil {
		globalStateMu.Unlock()
		return 0, fmt.Errorf("nessun tracking in corso")
	}

	watcher := globalWatcher
	sessionID := globalPendingSessionID
	globalIsTracking = false
	globalCurrentProject = nil
	globalWatcher = nil
	globalPendingSessionID = 0
	globalStateMu.Unlock()

	watcher.Stop()
	finalSeconds := watcher.GetTotalActiveSeconds()

	if sessionID > 0 {
		err := tracker.FinalizePendingTracking(a.db, sessionID, finalSeconds)
		if err != nil {
			return 0, err
		}
	}

	return finalSeconds, nil
}

// === IDLE TIME MANAGEMENT ===

// IdlePeriodData rappresenta un periodo di inattività pendente
type IdlePeriodData struct {
	HasPending bool   `json:"has_pending"`
	Minutes    int    `json:"minutes"`
	StartTime  string `json:"start_time"`
	EndTime    string `json:"end_time"`
}

// CheckIdlePeriod verifica se c'è un periodo idle pendente
func (a *App) CheckIdlePeriod() IdlePeriodData {
	globalStateMu.Lock()
	defer globalStateMu.Unlock()

	if globalWatcher == nil {
		return IdlePeriodData{HasPending: false}
	}

	idlePeriod := globalWatcher.GetPendingIdlePeriod()
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
	globalStateMu.Lock()
	defer globalStateMu.Unlock()

	if globalWatcher == nil {
		return fmt.Errorf("nessun watcher disponibile")
	}

	idlePeriod := globalWatcher.GetPendingIdlePeriod()
	if idlePeriod == nil {
		return fmt.Errorf("nessun periodo idle pendente")
	}
	if !globalIsTracking && globalPendingSessionID > 0 {
		if err := tracker.FinalizePendingTracking(a.db, globalPendingSessionID, globalWatcher.GetTotalActiveSeconds()); err != nil {
			return fmt.Errorf("impossibile completare il salvataggio del tracking: %w", err)
		}
		globalPendingSessionID = 0
	}

	if isBreak {
		// Registra come pausa (non viene salvato come sessione)
		globalWatcher.ClearPendingIdlePeriod()
		// Pulizia: se il tracking era già stato fermato (auto-stop), ferma il watcher
		if !globalIsTracking {
			globalWatcher.Stop()
			globalWatcher = nil
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

	globalWatcher.ClearPendingIdlePeriod()
	// Pulizia: se il tracking era già stato fermato (auto-stop), ferma il watcher
	if !globalIsTracking {
		globalWatcher.Stop()
		globalWatcher = nil
	}
	return nil
}

// SetIdleThreshold imposta la soglia di inattività in minuti
func (a *App) SetIdleThreshold(minutes int) error {
	if minutes < 1 || minutes > 1440 {
		return fmt.Errorf("la soglia di inattività deve essere compresa tra 1 e 1440 minuti")
	}
	if err := SetGlobalIdleThreshold(minutes); err != nil {
		return err
	}

	globalStateMu.Lock()
	watcher := globalWatcher
	globalStateMu.Unlock()
	if watcher != nil {
		watcher.SetIdleThreshold(minutes * 60)
	}
	return nil
}

// GetIdleThreshold ritorna la soglia di inattività corrente in minuti
func (a *App) GetIdleThreshold() int {
	return GetGlobalIdleThreshold()
}

// BringWindowToFront porta la finestra dell'applicazione in primo piano
func (a *App) BringWindowToFront() {
	// De-minimizza la finestra se minimizzata
	runtime.WindowUnminimise(a.ctx)
	// Mostra la finestra
	runtime.WindowShow(a.ctx)
	// Imposta always on top per forzare il primo piano
	runtime.WindowSetAlwaysOnTop(a.ctx, true)
}

// RestoreNormalWindow rimuove always on top (da chiamare dopo che l'utente ha interagito)
func (a *App) RestoreNormalWindow() {
	runtime.WindowSetAlwaysOnTop(a.ctx, false)
}
