package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"work-time-tracker-go/tracker"
)

// === EXPORT/IMPORT ===

type ExportProject struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	Archived    bool   `json:"archived"`
	ClosedAt    string `json:"closed_at,omitempty"`
	NoteText    string `json:"note_text,omitempty"`
	BoardURL    string `json:"board_url,omitempty"`
}

type ExportSession struct {
	ID           int     `json:"id"`
	AppName      string  `json:"app_name"`
	Seconds      int     `json:"seconds"`
	ProjectID    *int    `json:"project_id,omitempty"`
	SessionType  string  `json:"session_type"`
	ActivityType *string `json:"activity_type,omitempty"`
	Timestamp    string  `json:"timestamp"`
}

type ExportNote struct {
	ID        int    `json:"id"`
	ProjectID int    `json:"project_id"`
	NoteText  string `json:"note_text"`
	Timestamp string `json:"timestamp"`
}

type ExportActivityType struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	ColorVariant float64 `json:"color_variant"`
	Pattern      string  `json:"pattern"`
	DisplayOrder int     `json:"display_order"`
}

type ExportSetting struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type ExportDayEvent struct {
	Date      string  `json:"date"`
	EventType string  `json:"event_type"`
	Hours     float64 `json:"hours"`
}

// ExportDataResult rappresenta i dati esportati
type ExportDataResult struct {
	ExportDate    string               `json:"export_date"`
	Version       string               `json:"version"`
	Projects      []ExportProject      `json:"projects"`
	Sessions      []ExportSession      `json:"sessions"`
	Notes         []ExportNote         `json:"notes"`
	ActivityTypes []ExportActivityType `json:"activity_types"`
	Settings      []ExportSetting      `json:"settings,omitempty"`
	DayEvents     []ExportDayEvent     `json:"day_events,omitempty"`
}

// ExportData esporta tutti i dati
func (a *App) ExportData() (*ExportDataResult, error) {
	tx, err := a.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("errore avvio esportazione: %w", err)
	}
	defer tx.Rollback()

	result := &ExportDataResult{
		ExportDate: time.Now().Format("2006-01-02 15:04:05"),
		Version:    "2.3",
	}

	// Esporta progetti
	rows, err := tx.Query("SELECT id, name, COALESCE(description, ''), created_at, archived, COALESCE(closed_at, ''), COALESCE(note_text, ''), COALESCE(board_url, '') FROM projects")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var name, description, createdAt, closedAt, noteText, boardURL string
		var archived int
		if err := rows.Scan(&id, &name, &description, &createdAt, &archived, &closedAt, &noteText, &boardURL); err != nil {
			return nil, err
		}
		result.Projects = append(result.Projects, ExportProject{
			ID: id, Name: name, Description: description, CreatedAt: createdAt,
			Archived: archived == 1, ClosedAt: closedAt, NoteText: noteText, BoardURL: boardURL,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	// Esporta sessioni
	rows, err = tx.Query("SELECT id, app_name, seconds, project_id, session_type, activity_type, timestamp FROM sessions")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id, seconds int
		var appName, sessionType, timestamp string
		var projectID sql.NullInt64
		var activityType sql.NullString
		if err := rows.Scan(&id, &appName, &seconds, &projectID, &sessionType, &activityType, &timestamp); err != nil {
			return nil, err
		}
		session := ExportSession{ID: id, AppName: appName, Seconds: seconds, SessionType: sessionType, Timestamp: timestamp}
		if projectID.Valid {
			value := int(projectID.Int64)
			session.ProjectID = &value
		}
		if activityType.Valid {
			value := activityType.String
			session.ActivityType = &value
		}
		result.Sessions = append(result.Sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	// Esporta note
	rows, err = tx.Query("SELECT id, project_id, note_text, timestamp FROM notes")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id, projectID int
		var noteText, timestamp string
		if err := rows.Scan(&id, &projectID, &noteText, &timestamp); err != nil {
			return nil, err
		}
		result.Notes = append(result.Notes, ExportNote{ID: id, ProjectID: projectID, NoteText: noteText, Timestamp: timestamp})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	// Esporta tipi attività
	rows, err = tx.Query("SELECT id, name, color_variant, pattern, display_order FROM activity_types")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id, displayOrder int
		var name, pattern string
		var colorVariant float64
		if err := rows.Scan(&id, &name, &colorVariant, &pattern, &displayOrder); err != nil {
			return nil, err
		}
		result.ActivityTypes = append(result.ActivityTypes, ExportActivityType{
			ID: id, Name: name, ColorVariant: colorVariant, Pattern: pattern, DisplayOrder: displayOrder,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	rows, err = tx.Query("SELECT key, value FROM settings ORDER BY key")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var setting ExportSetting
		if err := rows.Scan(&setting.Key, &setting.Value); err != nil {
			return nil, err
		}
		result.Settings = append(result.Settings, setting)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	rows, err = tx.Query("SELECT date, event_type, hours FROM day_events ORDER BY date")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var event ExportDayEvent
		if err := rows.Scan(&event.Date, &event.EventType, &event.Hours); err != nil {
			return nil, err
		}
		result.DayEvents = append(result.DayEvents, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("errore completamento esportazione: %w", err)
	}
	return result, nil
}

// ImportData importa i dati da un backup
func (a *App) ImportData(data ExportDataResult) error {
	globalStateMu.Lock()
	defer globalStateMu.Unlock()
	if globalIsTracking || globalWatcher != nil {
		return fmt.Errorf("ferma il tracking e gestisci l'inattività pendente prima di importare un backup")
	}
	if err := validateBackupData(data); err != nil {
		return fmt.Errorf("backup non valido: %w", err)
	}
	if _, err := a.createAutomaticBackup(); err != nil {
		return fmt.Errorf("importazione annullata: impossibile creare il backup preventivo: %w", err)
	}

	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Elimina dati esistenti
	deleteStatements := []string{
		"DELETE FROM pending_tracking",
		"DELETE FROM notes",
		"DELETE FROM sessions",
		"DELETE FROM projects",
		"DELETE FROM activity_types",
	}
	for _, statement := range deleteStatements {
		if _, err := tx.Exec(statement); err != nil {
			return fmt.Errorf("errore preparazione import: %w", err)
		}
	}
	extendedBackup := data.Version == "2.0" || data.Version == "2.1" || data.Version == "2.2" || data.Version == "2.3" || len(data.Settings) > 0 || len(data.DayEvents) > 0
	if extendedBackup {
		if _, err := tx.Exec("DELETE FROM day_events"); err != nil {
			return fmt.Errorf("errore pulizia eventi: %w", err)
		}
		if _, err := tx.Exec("DELETE FROM settings"); err != nil {
			return fmt.Errorf("errore pulizia impostazioni: %w", err)
		}
	}

	// Mappa vecchi ID -> nuovi ID per i progetti
	projectIDMap := make(map[int]int64)

	// Importa progetti
	for _, p := range data.Projects {
		archived := 0
		if p.Archived {
			archived = 1
		}
		var closedAt interface{}
		if p.ClosedAt != "" {
			closedAt = p.ClosedAt
		}
		createdAt := p.CreatedAt
		if createdAt == "" {
			createdAt = time.Now().Format("2006-01-02 15:04:05")
		}

		result, err := tx.Exec(
			"INSERT INTO projects (name, description, created_at, archived, closed_at, note_text, board_url) VALUES (?, ?, ?, ?, ?, ?, ?)",
			p.Name, p.Description, createdAt, archived, closedAt, p.NoteText, p.BoardURL,
		)
		if err != nil {
			return err
		}
		newID, err := result.LastInsertId()
		if err != nil {
			return err
		}
		projectIDMap[p.ID] = newID
	}

	// Importa sessioni
	for _, s := range data.Sessions {
		var projectID *int64
		if s.ProjectID != nil {
			if newPID, exists := projectIDMap[*s.ProjectID]; exists {
				projectID = &newPID
			}
		}

		_, err := tx.Exec(
			"INSERT INTO sessions (app_name, seconds, project_id, session_type, activity_type, timestamp) VALUES (?, ?, ?, ?, ?, ?)",
			s.AppName, s.Seconds, projectID, s.SessionType, s.ActivityType, s.Timestamp,
		)
		if err != nil {
			return err
		}
	}

	// Importa note
	for _, n := range data.Notes {
		newProjectID, exists := projectIDMap[n.ProjectID]
		if !exists {
			continue
		}

		_, err := tx.Exec(
			"INSERT INTO notes (project_id, note_text, timestamp) VALUES (?, ?, ?)",
			newProjectID, n.NoteText, n.Timestamp,
		)
		if err != nil {
			return err
		}
	}

	// Importa tipi attività
	for _, at := range data.ActivityTypes {
		pattern := at.Pattern
		if pattern == "" {
			pattern = "solid"
		}

		_, err := tx.Exec(
			"INSERT INTO activity_types (name, color_variant, pattern, display_order) VALUES (?, ?, ?, ?)",
			at.Name, at.ColorVariant, pattern, at.DisplayOrder,
		)
		if err != nil {
			return err
		}
	}

	if extendedBackup {
		for _, setting := range data.Settings {
			if _, err := tx.Exec("INSERT INTO settings (key, value) VALUES (?, ?)", setting.Key, setting.Value); err != nil {
				return err
			}
		}
		for _, event := range data.DayEvents {
			if _, err := tx.Exec("INSERT INTO day_events (date, event_type, hours) VALUES (?, ?, ?)", event.Date, event.EventType, event.Hours); err != nil {
				return err
			}
		}
		if _, err := tx.Exec("INSERT OR IGNORE INTO settings (key, value) VALUES ('auto_start', 'false')"); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	globalDB = a.db
	globalIsTracking = false
	globalCurrentProject = nil
	globalWatcher = nil
	globalPendingSessionID = 0
	globalIdleThreshold = 5
	if value, err := tracker.GetSetting(a.db, "idle_threshold"); err == nil && value != "" {
		var threshold int
		if _, err := fmt.Sscanf(value, "%d", &threshold); err == nil && threshold > 0 {
			globalIdleThreshold = threshold
		}
	}
	return nil
}

func (a *App) createAutomaticBackup() (string, error) {
	current, err := a.ExportData()
	if err != nil {
		return "", err
	}
	jsonData, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return "", err
	}

	var sequence int
	var databaseName, databasePath string
	if err := a.db.QueryRow("PRAGMA database_list").Scan(&sequence, &databaseName, &databasePath); err != nil {
		return "", err
	}
	if databasePath == "" {
		return "", fmt.Errorf("percorso del database non disponibile")
	}
	backupDir := filepath.Join(filepath.Dir(databasePath), "backups")
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return "", err
	}
	backupPath := filepath.Join(backupDir, "pre-import-"+time.Now().Format("20060102-150405.000000000")+".json")
	temporaryPath := backupPath + ".tmp"
	if err := os.WriteFile(temporaryPath, jsonData, 0600); err != nil {
		return "", err
	}
	if err := os.Rename(temporaryPath, backupPath); err != nil {
		_ = os.Remove(temporaryPath)
		return "", err
	}
	return backupPath, nil
}
