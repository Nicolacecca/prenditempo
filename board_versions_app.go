package main

import (
	"database/sql"
	"fmt"
	"strings"

	"work-time-tracker-go/tracker"
)

const maxNamedBoardVersions = 50

type BoardVersionData struct {
	ID          int64  `json:"id"`
	ProjectID   int    `json:"project_id"`
	Name        string `json:"name"`
	IsAutomatic bool   `json:"is_automatic"`
	CreatedAt   string `json:"created_at"`
}

func (a *App) CreateBoardVersion(projectID int, name string) (*BoardVersionData, error) {
	name = strings.TrimSpace(name)
	if projectID <= 0 || name == "" || len([]rune(name)) > 80 {
		return nil, fmt.Errorf("nome versione non valido")
	}
	var count int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM board_versions WHERE project_id = ? AND is_automatic = 0`, projectID).Scan(&count); err != nil {
		return nil, err
	}
	if count >= maxNamedBoardVersions {
		return nil, fmt.Errorf("limite di %d versioni nominate raggiunto: eliminane una prima di continuare", maxNamedBoardVersions)
	}
	board, err := tracker.GetProjectBoard(a.db, projectID)
	if err != nil {
		return nil, err
	}
	if board == nil {
		return nil, fmt.Errorf("lavagna non trovata")
	}
	id, err := tracker.CreateBoardVersion(a.db, tracker.BoardVersion{
		ProjectID: projectID, Name: name, DocumentJSON: board.DocumentJSON, ViewportJSON: board.ViewportJSON,
	})
	if err != nil {
		return nil, err
	}
	version, err := tracker.GetBoardVersion(a.db, projectID, id)
	if err != nil {
		return nil, err
	}
	return boardVersionData(version), nil
}

func (a *App) ListBoardVersions(projectID int) ([]BoardVersionData, error) {
	if projectID <= 0 {
		return nil, fmt.Errorf("ID progetto non valido")
	}
	versions, err := tracker.ListBoardVersions(a.db, projectID)
	if err != nil {
		return nil, err
	}
	result := make([]BoardVersionData, 0, len(versions))
	for index := range versions {
		result = append(result, *boardVersionData(&versions[index]))
	}
	return result, nil
}

func (a *App) GetBoardVersion(projectID int, versionID int64) (*ProjectBoardData, error) {
	if projectID <= 0 || versionID <= 0 {
		return nil, fmt.Errorf("versione non valida")
	}
	version, err := tracker.GetBoardVersion(a.db, projectID, versionID)
	if err != nil {
		return nil, err
	}
	if version == nil {
		return nil, sql.ErrNoRows
	}
	return &ProjectBoardData{
		ProjectID: projectID, SchemaVersion: currentBoardSchemaVersion,
		DocumentJSON: version.DocumentJSON, ViewportJSON: version.ViewportJSON,
		LegacyNoteImported: true, UpdatedAt: version.CreatedAt,
	}, nil
}

func (a *App) DeleteBoardVersion(projectID int, versionID int64) error {
	if projectID <= 0 || versionID <= 0 {
		return fmt.Errorf("versione non valida")
	}
	return tracker.DeleteBoardVersion(a.db, projectID, versionID)
}

func (a *App) RestoreBoardVersion(projectID int, versionID int64) (*ProjectBoardData, error) {
	if projectID <= 0 || versionID <= 0 {
		return nil, fmt.Errorf("versione non valida")
	}
	target, err := tracker.GetBoardVersion(a.db, projectID, versionID)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, fmt.Errorf("versione lavagna non trovata")
	}
	current, err := tracker.GetProjectBoard(a.db, projectID)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, fmt.Errorf("lavagna non trovata")
	}
	if current.DocumentJSON != target.DocumentJSON {
		if _, err := tracker.CreateBoardVersion(a.db, tracker.BoardVersion{
			ProjectID: projectID, Name: "Prima del ripristino", IsAutomatic: true,
			DocumentJSON: current.DocumentJSON, ViewportJSON: current.ViewportJSON,
		}); err != nil {
			return nil, err
		}
	}
	if err := tracker.SaveProjectBoard(a.db, tracker.ProjectBoard{
		ProjectID: projectID, SchemaVersion: currentBoardSchemaVersion,
		DocumentJSON: target.DocumentJSON, ViewportJSON: target.ViewportJSON,
		LegacyNoteImported: true,
	}); err != nil {
		return nil, err
	}
	return &ProjectBoardData{
		ProjectID: projectID, SchemaVersion: currentBoardSchemaVersion,
		DocumentJSON: target.DocumentJSON, ViewportJSON: target.ViewportJSON,
		LegacyNoteImported: true, UpdatedAt: target.CreatedAt,
	}, nil
}

func boardVersionData(version *tracker.BoardVersion) *BoardVersionData {
	return &BoardVersionData{
		ID: version.ID, ProjectID: version.ProjectID, Name: version.Name,
		IsAutomatic: version.IsAutomatic, CreatedAt: version.CreatedAt,
	}
}
