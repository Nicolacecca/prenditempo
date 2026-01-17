package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
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
	return tracker.CreaProgetto(a.db, name, description)
}

// ArchiveProject archivia un progetto
func (a *App) ArchiveProject(projectID int) error {
	return tracker.ArchivaProgetto(a.db, projectID)
}

// ReactivateProject riattiva un progetto archiviato
func (a *App) ReactivateProject(projectID int) error {
	return tracker.RiattivaProgetto(a.db, projectID)
}

// DeleteProject elimina un progetto
func (a *App) DeleteProject(projectID int) error {
	return tracker.EliminaProgettoById(a.db, projectID)
}

// UpdateProject aggiorna nome e descrizione di un progetto
func (a *App) UpdateProject(projectID int, name, description string) error {
	return tracker.AggiornaProgetto(a.db, projectID, name, description)
}

// UpdateProjectNote aggiorna la nota markdown di un progetto
func (a *App) UpdateProjectNote(projectID int, noteText string) error {
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
	return tracker.CreaSessione(a.db, appName, seconds, projectID, sessionType, activityType, timestamp)
}

// UpdateSessionDuration aggiorna la durata di una sessione
func (a *App) UpdateSessionDuration(sessionID, seconds int) error {
	return tracker.AggiornaDurataSessione(a.db, sessionID, seconds)
}

// UpdateSessionActivityType aggiorna il tipo di attività di una sessione
func (a *App) UpdateSessionActivityType(sessionID int, activityType *string) error {
	return tracker.AggiornaActivityType(a.db, sessionID, activityType)
}

// DeleteSession elimina una sessione
func (a *App) DeleteSession(sessionID int) error {
	return tracker.EliminaSessione(a.db, sessionID)
}

// SplitSession divide una sessione in due parti
func (a *App) SplitSession(sessionID, firstPartSeconds int, firstActivityType, secondActivityType *string) error {
	return tracker.DividiSessione(a.db, sessionID, firstPartSeconds, firstActivityType, secondActivityType)
}

// UpdateSessionComplete aggiorna timestamp, durata e tipo attività di una sessione
func (a *App) UpdateSessionComplete(sessionID int, newTimestamp string, newSeconds int, activityType *string) error {
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
	return tracker.CreaTipoAttivita(a.db, name, colorVariant, pattern, displayOrder)
}

// UpdateActivityType aggiorna un tipo di attività
func (a *App) UpdateActivityType(id int, name string, colorVariant float64, pattern string, displayOrder int) error {
	return tracker.AggiornaTipoAttivita(a.db, id, name, colorVariant, pattern, displayOrder)
}

// DeleteActivityType elimina un tipo di attività
func (a *App) DeleteActivityType(id int) error {
	return tracker.EliminaTipoAttivita(a.db, id)
}

// ReorderActivityTypes aggiorna l'ordine dei tipi di attività
func (a *App) ReorderActivityTypes(updates []map[string]int) error {
	for _, u := range updates {
		if err := tracker.AggiornaOrdineTipoAttivita(a.db, u["id"], u["display_order"]); err != nil {
			return err
		}
	}
	return nil
}

// === TRACKING ===

// TrackingState rappresenta lo stato del tracking
type TrackingState struct {
	IsTracking      bool    `json:"is_tracking"`
	ProjectID       *int    `json:"project_id,omitempty"`
	ProjectName     string  `json:"project_name,omitempty"`
	ActivityType    *string `json:"activity_type,omitempty"`
	StartTime       string  `json:"start_time,omitempty"`
	ElapsedSeconds  int     `json:"elapsed_seconds"`
	SessionID       int64   `json:"session_id,omitempty"`
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
	project, err := tracker.TrovaProgettoById(a.db, projectID)
	if err != nil {
		return err
	}

	// Crea sessione pendente
	startTime := time.Now().Format("2006-01-02 15:04:05")
	sessionID, err := tracker.StartPendingTracking(a.db, &projectID, activityType, startTime)
	if err != nil {
		return err
	}

	// Crea watcher
	watcher := tracker.NewTimeWatcher()
	idleMinutes := GetGlobalIdleThreshold()
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
		// Invia notifica toast Windows
		a.ShowIdleNotification(minutes)
		// Prova anche a portare la finestra in primo piano
		a.BringWindowToFront()
	})

	watcher.Start(5)

	// Aggiorna stato globale
	SetGlobalTrackingState(watcher, project, true, sessionID)

	return nil
}

// autoStopOnIdle ferma automaticamente il tracking quando viene rilevato idle
func (a *App) autoStopOnIdle(watcher *tracker.TimeWatcher, sessionID int64) {
	globalStateMu.Lock()
	defer globalStateMu.Unlock()

	// Verifica che il tracking sia ancora attivo con questo watcher
	if !globalIsTracking || globalWatcher != watcher {
		fmt.Println("[IDLE-AUTOSTOP] Tracking già fermato o watcher diverso, skip")
		return
	}

	// Ferma il watcher (non blocca perché chiamato dalla goroutine del watcher stesso)
	// NON chiamare watcher.Stop() qui perché siamo già dentro la goroutine
	finalSeconds := watcher.GetTotalActiveSeconds()

	// Salva la sessione finale nel database
	if sessionID > 0 && finalSeconds > 0 {
		err := tracker.FinalizePendingTracking(a.db, sessionID, finalSeconds)
		if err != nil {
			fmt.Printf("[IDLE-AUTOSTOP] Errore salvataggio sessione: %v\n", err)
		} else {
			fmt.Printf("[IDLE-AUTOSTOP] Sessione salvata: %d secondi (%d min)\n", finalSeconds, finalSeconds/60)
		}
	}

	// Reset stato globale (tracking fermato) MA mantieni il watcher per il pending idle
	globalIsTracking = false
	globalCurrentProject = nil
	// NON azzerare globalWatcher - serve per rilevare quando l'utente torna e mostrare il modale idle
	// globalWatcher = nil
	globalPendingSessionID = 0

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
	defer globalStateMu.Unlock()

	if !globalIsTracking || globalWatcher == nil {
		return 0, fmt.Errorf("nessun tracking in corso")
	}

	globalWatcher.Stop()
	finalSeconds := globalWatcher.GetTotalActiveSeconds()

	if globalPendingSessionID > 0 {
		err := tracker.FinalizePendingTracking(a.db, globalPendingSessionID, finalSeconds)
		if err != nil {
			return 0, err
		}
	}

	// Reset stato
	globalIsTracking = false
	globalCurrentProject = nil
	globalWatcher = nil
	globalPendingSessionID = 0

	return finalSeconds, nil
}

// === STATISTICHE ===

// GetTodayStats restituisce le statistiche di oggi
func (a *App) GetTodayStats() (map[string]int, error) {
	return tracker.CaricaSessioniOggi(a.db)
}

// GetWeekStats restituisce le statistiche della settimana
func (a *App) GetWeekStats() (map[string]int, error) {
	return tracker.CaricaSessioniSettimana(a.db)
}

// GetMonthStats restituisce le statistiche del mese
func (a *App) GetMonthStats() (map[string]int, error) {
	return tracker.CaricaSessioniMese(a.db)
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

	// Crea una sessione per il tempo idle
	seconds := idlePeriod.Duration // Duration è già in secondi
	timestamp := idlePeriod.StartTime.Format("2006-01-02 15:04:05")
	err := tracker.CreaSessione(a.db, "Tempo Idle", seconds, &projectID, "off-computer", nil, timestamp)
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
func (a *App) SetIdleThreshold(minutes int) {
	if minutes < 1 {
		minutes = 1
	}
	SetGlobalIdleThreshold(minutes)

	// Se il watcher è attivo, aggiorna anche la sua soglia
	if globalWatcher != nil {
		globalWatcher.SetIdleThreshold(minutes * 60)
	}
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

// === EXPORT/IMPORT ===

// ExportDataResult rappresenta i dati esportati
type ExportDataResult struct {
	ExportDate    string                   `json:"export_date"`
	Version       string                   `json:"version"`
	Projects      []map[string]interface{} `json:"projects"`
	Sessions      []map[string]interface{} `json:"sessions"`
	Notes         []map[string]interface{} `json:"notes"`
	ActivityTypes []map[string]interface{} `json:"activity_types"`
}

// ExportData esporta tutti i dati
func (a *App) ExportData() (*ExportDataResult, error) {
	result := &ExportDataResult{
		ExportDate: time.Now().Format("2006-01-02 15:04:05"),
		Version:    "1.0",
	}

	// Esporta progetti
	rows, err := a.db.Query("SELECT id, name, description, created_at, archived, COALESCE(closed_at, '') FROM projects")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var name, description, createdAt, closedAt string
		var archived int
		rows.Scan(&id, &name, &description, &createdAt, &archived, &closedAt)
		project := map[string]interface{}{
			"id":          id,
			"name":        name,
			"description": description,
			"created_at":  createdAt,
			"archived":    archived == 1,
		}
		if closedAt != "" {
			project["closed_at"] = closedAt
		}
		result.Projects = append(result.Projects, project)
	}

	// Esporta sessioni
	rows, err = a.db.Query("SELECT id, app_name, seconds, project_id, session_type, activity_type, timestamp FROM sessions")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id, seconds int
		var appName, sessionType, timestamp string
		var projectID sql.NullInt64
		var activityType sql.NullString
		rows.Scan(&id, &appName, &seconds, &projectID, &sessionType, &activityType, &timestamp)
		session := map[string]interface{}{
			"id":           id,
			"app_name":     appName,
			"seconds":      seconds,
			"session_type": sessionType,
			"timestamp":    timestamp,
		}
		if projectID.Valid {
			session["project_id"] = projectID.Int64
		}
		if activityType.Valid {
			session["activity_type"] = activityType.String
		}
		result.Sessions = append(result.Sessions, session)
	}

	// Esporta note
	rows, err = a.db.Query("SELECT id, project_id, note_text, timestamp FROM notes")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id, projectID int
		var noteText, timestamp string
		rows.Scan(&id, &projectID, &noteText, &timestamp)
		result.Notes = append(result.Notes, map[string]interface{}{
			"id":         id,
			"project_id": projectID,
			"note_text":  noteText,
			"timestamp":  timestamp,
		})
	}

	// Esporta tipi attività
	rows, err = a.db.Query("SELECT id, name, color_variant, pattern, display_order FROM activity_types")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id, displayOrder int
		var name, pattern string
		var colorVariant float64
		rows.Scan(&id, &name, &colorVariant, &pattern, &displayOrder)
		result.ActivityTypes = append(result.ActivityTypes, map[string]interface{}{
			"id":            id,
			"name":          name,
			"color_variant": colorVariant,
			"pattern":       pattern,
			"display_order": displayOrder,
		})
	}

	return result, nil
}

// ImportData importa i dati da un backup
func (a *App) ImportData(data ExportDataResult) error {
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}

	// Elimina dati esistenti
	tx.Exec("DELETE FROM pending_tracking")
	tx.Exec("DELETE FROM notes")
	tx.Exec("DELETE FROM sessions")
	tx.Exec("DELETE FROM projects")
	tx.Exec("DELETE FROM activity_types")

	// Mappa vecchi ID -> nuovi ID per i progetti
	projectIDMap := make(map[int]int64)

	// Importa progetti
	for _, p := range data.Projects {
		name := p["name"].(string)
		description := ""
		if d, ok := p["description"].(string); ok {
			description = d
		}
		archived := 0
		if a, ok := p["archived"].(bool); ok && a {
			archived = 1
		}
		var closedAt interface{}
		if c, ok := p["closed_at"].(string); ok && c != "" {
			closedAt = c
		}

		result, err := tx.Exec(
			"INSERT INTO projects (name, description, archived, closed_at) VALUES (?, ?, ?, ?)",
			name, description, archived, closedAt,
		)
		if err != nil {
			tx.Rollback()
			return err
		}
		newID, _ := result.LastInsertId()
		oldID := int(p["id"].(float64))
		projectIDMap[oldID] = newID
	}

	// Importa sessioni
	for _, s := range data.Sessions {
		appName := s["app_name"].(string)
		seconds := int(s["seconds"].(float64))
		sessionType := s["session_type"].(string)
		timestamp := s["timestamp"].(string)

		var projectID *int64
		if pid, ok := s["project_id"].(float64); ok {
			if newPID, exists := projectIDMap[int(pid)]; exists {
				projectID = &newPID
			}
		}

		var activityType *string
		if at, ok := s["activity_type"].(string); ok {
			activityType = &at
		}

		_, err := tx.Exec(
			"INSERT INTO sessions (app_name, seconds, project_id, session_type, activity_type, timestamp) VALUES (?, ?, ?, ?, ?, ?)",
			appName, seconds, projectID, sessionType, activityType, timestamp,
		)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	// Importa note
	for _, n := range data.Notes {
		oldProjectID := int(n["project_id"].(float64))
		noteText := n["note_text"].(string)
		timestamp := n["timestamp"].(string)

		newProjectID, exists := projectIDMap[oldProjectID]
		if !exists {
			continue
		}

		_, err := tx.Exec(
			"INSERT INTO notes (project_id, note_text, timestamp) VALUES (?, ?, ?)",
			newProjectID, noteText, timestamp,
		)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	// Importa tipi attività
	for _, at := range data.ActivityTypes {
		name := at["name"].(string)
		colorVariant := at["color_variant"].(float64)
		pattern := "solid"
		if p, ok := at["pattern"].(string); ok {
			pattern = p
		}
		displayOrder := 0
		if d, ok := at["display_order"].(float64); ok {
			displayOrder = int(d)
		}

		_, err := tx.Exec(
			"INSERT INTO activity_types (name, color_variant, pattern, display_order) VALUES (?, ?, ?, ?)",
			name, colorVariant, pattern, displayOrder,
		)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

// === SALVATAGGIO REPORT ===

// SaveReportJSON salva il report in formato JSON
func (a *App) SaveReportJSON(projectID int) (string, error) {
	// Genera il report
	report, err := tracker.GeneraReportChiusura(a.db, projectID)
	if err != nil {
		return "", err
	}

	// Aggiungi metadati per reimportazione
	report["export_type"] = "project_report"
	report["export_date"] = time.Now().Format("2006-01-02 15:04:05")
	report["version"] = "1.0"

	// Recupera sessioni del progetto per backup completo
	sessions, err := tracker.CaricaSessioniDettagliate(a.db, "2000-01-01", "2099-12-31")
	if err == nil {
		var projectSessions []map[string]interface{}
		for _, s := range sessions {
			if s.ProjectID != nil && *s.ProjectID == projectID {
				projectSessions = append(projectSessions, map[string]interface{}{
					"id":            s.ID,
					"app_name":      s.AppName,
					"seconds":       s.Seconds,
					"project_id":    s.ProjectID,
					"session_type":  s.SessionType,
					"activity_type": s.ActivityType,
					"timestamp":     s.Timestamp,
				})
			}
		}
		report["sessions"] = projectSessions
	}

	// Converti in JSON
	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}

	// Chiedi all'utente dove salvare
	projectName := report["project_name"].(string)
	defaultName := fmt.Sprintf("Report_%s_%s.json", projectName, time.Now().Format("2006-01-02"))

	filePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: defaultName,
		Title:           "Salva Report JSON",
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON Files (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil {
		return "", err
	}
	if filePath == "" {
		return "", nil // Utente ha annullato
	}

	// Salva il file
	err = os.WriteFile(filePath, jsonData, 0644)
	if err != nil {
		return "", err
	}

	return filePath, nil
}

// SaveReportText salva il report in formato testo (per PDF useremo JS)
func (a *App) SaveReportText(projectID int) (string, error) {
	// Genera il report
	report, err := tracker.GeneraReportChiusura(a.db, projectID)
	if err != nil {
		return "", err
	}

	projectName := report["project_name"].(string)
	projectDesc := ""
	if desc, ok := report["project_description"].(string); ok {
		projectDesc = desc
	}
	totalHours := 0.0
	if h, ok := report["total_hours"].(float64); ok {
		totalHours = h
	}
	startDate := ""
	if s, ok := report["start_date"].(string); ok {
		startDate = s
	}
	endDate := ""
	if e, ok := report["end_date"].(string); ok {
		endDate = e
	}
	closedAt := ""
	if c, ok := report["closed_at"].(string); ok {
		closedAt = c
	}

	// Costruisci il testo del report
	text := fmt.Sprintf("REPORT PROGETTO: %s\n", projectName)
	text += fmt.Sprintf("================================\n\n")
	if projectDesc != "" {
		text += fmt.Sprintf("Descrizione: %s\n\n", projectDesc)
	}
	text += fmt.Sprintf("Periodo: %s - %s\n", startDate, endDate)
	text += fmt.Sprintf("Chiuso il: %s\n\n", closedAt)
	text += fmt.Sprintf("TOTALE ORE TRACCIATE: %.2f ore\n\n", totalHours)

	// Suddivisione per tipo attività
	if breakdown, ok := report["activity_breakdown"].(map[string]float64); ok && len(breakdown) > 0 {
		text += "SUDDIVISIONE PER TIPO ATTIVITÀ:\n"
		text += "--------------------------------\n"
		for activity, hours := range breakdown {
			text += fmt.Sprintf("  %s: %.2f ore\n", activity, hours)
		}
	}

	text += fmt.Sprintf("\n\nGenerato da PrendiTempo il %s\n", time.Now().Format("02/01/2006 15:04"))

	// Chiedi all'utente dove salvare
	defaultName := fmt.Sprintf("Report_%s_%s.txt", projectName, time.Now().Format("2006-01-02"))

	filePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: defaultName,
		Title:           "Salva Report",
		Filters: []runtime.FileFilter{
			{DisplayName: "Text Files (*.txt)", Pattern: "*.txt"},
		},
	})
	if err != nil {
		return "", err
	}
	if filePath == "" {
		return "", nil // Utente ha annullato
	}

	// Salva il file
	err = os.WriteFile(filePath, []byte(text), 0644)
	if err != nil {
		return "", err
	}

	return filePath, nil
}

// ImportProjectJSON importa un progetto da file JSON
func (a *App) ImportProjectJSON() (string, error) {
	// Chiedi all'utente di selezionare il file
	filePath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Seleziona Report JSON da Importare",
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON Files (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil {
		return "", err
	}
	if filePath == "" {
		return "", nil // Utente ha annullato
	}

	// Leggi il file
	jsonData, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("errore lettura file: %v", err)
	}

	// Parse del JSON
	var reportData map[string]interface{}
	if err := json.Unmarshal(jsonData, &reportData); err != nil {
		return "", fmt.Errorf("errore parsing JSON: %v", err)
	}

	// Verifica che sia un report valido
	exportType, ok := reportData["export_type"].(string)
	if !ok || exportType != "project_report" {
		return "", fmt.Errorf("file non valido: non è un report di progetto PrendiTempo")
	}

	// Estrai dati progetto
	projectName, ok := reportData["project_name"].(string)
	if !ok || projectName == "" {
		return "", fmt.Errorf("nome progetto mancante nel file")
	}

	projectDesc := ""
	if desc, ok := reportData["project_description"].(string); ok {
		projectDesc = desc
	}

	// Verifica se il progetto esiste già
	existingProject, _ := tracker.TrovaProgetto(a.db, projectName)
	if existingProject != nil {
		// Aggiungi suffisso per evitare conflitti
		projectName = fmt.Sprintf("%s (Importato %s)", projectName, time.Now().Format("02-01-2006 15:04"))
	}

	// Crea il nuovo progetto
	tx, err := a.db.Begin()
	if err != nil {
		return "", err
	}

	result, err := tx.Exec(
		"INSERT INTO projects (name, description, archived, closed_at) VALUES (?, ?, 0, NULL)",
		projectName, projectDesc,
	)
	if err != nil {
		tx.Rollback()
		return "", fmt.Errorf("errore creazione progetto: %v", err)
	}

	newProjectID, err := result.LastInsertId()
	if err != nil {
		tx.Rollback()
		return "", err
	}

	// Importa sessioni
	sessionsImported := 0
	if sessions, ok := reportData["sessions"].([]interface{}); ok {
		for _, s := range sessions {
			session, ok := s.(map[string]interface{})
			if !ok {
				continue
			}

			appName := ""
			if an, ok := session["app_name"].(string); ok {
				appName = an
			}

			seconds := 0
			if sec, ok := session["seconds"].(float64); ok {
				seconds = int(sec)
			}

			sessionType := "manual"
			if st, ok := session["session_type"].(string); ok {
				sessionType = st
			}

			var activityType *string
			if at, ok := session["activity_type"].(string); ok && at != "" {
				activityType = &at
			}

			timestamp := time.Now().Format("2006-01-02 15:04:05")
			if ts, ok := session["timestamp"].(string); ok && ts != "" {
				timestamp = ts
			}

			_, err := tx.Exec(
				"INSERT INTO sessions (app_name, seconds, project_id, session_type, activity_type, timestamp) VALUES (?, ?, ?, ?, ?, ?)",
				appName, seconds, newProjectID, sessionType, activityType, timestamp,
			)
			if err == nil {
				sessionsImported++
			}
		}
	}

	// Importa note
	notesImported := 0
	if notes, ok := reportData["notes"].([]interface{}); ok {
		for _, n := range notes {
			note, ok := n.(map[string]interface{})
			if !ok {
				continue
			}

			noteText := ""
			if nt, ok := note["note_text"].(string); ok {
				noteText = nt
			}

			timestamp := time.Now().Format("2006-01-02 15:04:05")
			if ts, ok := note["timestamp"].(string); ok && ts != "" {
				timestamp = ts
			}

			_, err := tx.Exec(
				"INSERT INTO notes (project_id, note_text, timestamp) VALUES (?, ?, ?)",
				newProjectID, noteText, timestamp,
			)
			if err == nil {
				notesImported++
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}

	return fmt.Sprintf("Progetto '%s' importato con successo!\n%d sessioni e %d note ripristinate.", projectName, sessionsImported, notesImported), nil
}
