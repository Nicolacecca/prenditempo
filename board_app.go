package main

import (
	"encoding/json"
	"fmt"
	"time"

	"work-time-tracker-go/tracker"
)

const (
	currentBoardSchemaVersion = 1
	maxBoardDocumentBytes     = 10 * 1024 * 1024
	maxBoardViewportBytes     = 64 * 1024
)

type ProjectBoardData struct {
	ProjectID          int    `json:"project_id"`
	SchemaVersion      int    `json:"schema_version"`
	DocumentJSON       string `json:"document_json"`
	ViewportJSON       string `json:"viewport_json"`
	LegacyNoteImported bool   `json:"legacy_note_imported"`
	UpdatedAt          string `json:"updated_at,omitempty"`
}

func (a *App) GetProjectBoard(projectID int) (*ProjectBoardData, error) {
	if projectID <= 0 {
		return nil, fmt.Errorf("ID progetto non valido")
	}
	board, err := tracker.GetProjectBoard(a.db, projectID)
	if err != nil {
		return nil, err
	}
	if board == nil {
		var exists int
		if err := a.db.QueryRow(`SELECT 1 FROM projects WHERE id = ?`, projectID).Scan(&exists); err != nil {
			return nil, fmt.Errorf("progetto non trovato")
		}
		return &ProjectBoardData{
			ProjectID:     projectID,
			SchemaVersion: currentBoardSchemaVersion,
			DocumentJSON:  "{}",
			ViewportJSON:  "{}",
		}, nil
	}
	return &ProjectBoardData{
		ProjectID:          board.ProjectID,
		SchemaVersion:      board.SchemaVersion,
		DocumentJSON:       board.DocumentJSON,
		ViewportJSON:       board.ViewportJSON,
		LegacyNoteImported: board.LegacyNoteImported,
		UpdatedAt:          board.UpdatedAt,
	}, nil
}

func (a *App) SaveProjectBoard(board ProjectBoardData) error {
	if board.ProjectID <= 0 || board.SchemaVersion != currentBoardSchemaVersion {
		return fmt.Errorf("dati lavagna non validi")
	}
	if len(board.DocumentJSON) > maxBoardDocumentBytes || !json.Valid([]byte(board.DocumentJSON)) {
		return fmt.Errorf("documento lavagna non valido o superiore a 10 MiB")
	}
	if len(board.ViewportJSON) > maxBoardViewportBytes || !json.Valid([]byte(board.ViewportJSON)) {
		return fmt.Errorf("visuale lavagna non valida")
	}
	if err := tracker.CreateAutomaticBoardVersionIfDue(a.db, board.ProjectID, board.DocumentJSON, 15*time.Minute, 20); err != nil {
		return fmt.Errorf("impossibile proteggere la versione precedente: %w", err)
	}
	return tracker.SaveProjectBoard(a.db, tracker.ProjectBoard{
		ProjectID:          board.ProjectID,
		SchemaVersion:      board.SchemaVersion,
		DocumentJSON:       board.DocumentJSON,
		ViewportJSON:       board.ViewportJSON,
		LegacyNoteImported: board.LegacyNoteImported,
	})
}
