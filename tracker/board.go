package tracker

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type ProjectBoard struct {
	ProjectID          int
	SchemaVersion      int
	DocumentJSON       string
	ViewportJSON       string
	LegacyNoteImported bool
	UpdatedAt          string
}

type BoardVersion struct {
	ID           int64
	ProjectID    int
	Name         string
	IsAutomatic  bool
	DocumentJSON string
	ViewportJSON string
	CreatedAt    string
}

func GetProjectBoard(db *sql.DB, projectID int) (*ProjectBoard, error) {
	var board ProjectBoard
	var imported int
	err := db.QueryRow(`
		SELECT project_id, schema_version, document_json, viewport_json,
		       legacy_note_imported, updated_at
		FROM project_boards
		WHERE project_id = ?`, projectID,
	).Scan(
		&board.ProjectID,
		&board.SchemaVersion,
		&board.DocumentJSON,
		&board.ViewportJSON,
		&imported,
		&board.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("errore lettura lavagna: %w", err)
	}
	board.LegacyNoteImported = imported == 1
	return &board, nil
}

func SaveProjectBoard(db *sql.DB, board ProjectBoard) error {
	imported := 0
	if board.LegacyNoteImported {
		imported = 1
	}
	result, err := db.Exec(`
		INSERT INTO project_boards (
			project_id, schema_version, document_json, viewport_json,
			legacy_note_imported, updated_at
		) VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(project_id) DO UPDATE SET
			schema_version = excluded.schema_version,
			document_json = excluded.document_json,
			viewport_json = excluded.viewport_json,
			legacy_note_imported = excluded.legacy_note_imported,
			updated_at = CURRENT_TIMESTAMP`,
		board.ProjectID,
		board.SchemaVersion,
		board.DocumentJSON,
		board.ViewportJSON,
		imported,
	)
	if err != nil {
		return fmt.Errorf("errore salvataggio lavagna: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("errore verifica salvataggio lavagna: %w", err)
	}
	if rows != 1 {
		return fmt.Errorf("lavagna non salvata")
	}
	return nil
}

func CreateAutomaticBoardVersionIfDue(db *sql.DB, projectID int, newDocumentJSON string, interval time.Duration, limit int) error {
	var documentJSON, viewportJSON string
	err := db.QueryRow(`SELECT document_json, viewport_json FROM project_boards WHERE project_id = ?`, projectID).Scan(&documentJSON, &viewportJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("errore lettura stato precedente lavagna: %w", err)
	}
	if documentJSON == newDocumentJSON {
		return nil
	}
	minutes := int(interval.Minutes())
	if minutes < 1 {
		minutes = 1
	}
	var recent int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM board_versions
		WHERE project_id = ? AND is_automatic = 1
		  AND created_at >= datetime('now', ?)`, projectID, fmt.Sprintf("-%d minutes", minutes)).Scan(&recent); err != nil {
		return fmt.Errorf("errore verifica cronologia lavagna: %w", err)
	}
	if recent > 0 {
		return nil
	}
	var same int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM board_versions
		WHERE project_id = ? AND document_json = ?
		ORDER BY created_at DESC LIMIT 1`, projectID, documentJSON).Scan(&same); err != nil {
		return fmt.Errorf("errore confronto cronologia lavagna: %w", err)
	}
	if same == 0 {
		if _, err := CreateBoardVersion(db, BoardVersion{
			ProjectID: projectID, Name: "Salvataggio automatico", IsAutomatic: true,
			DocumentJSON: documentJSON, ViewportJSON: viewportJSON,
		}); err != nil {
			return err
		}
	}
	if limit > 0 {
		_, err = db.Exec(`
			DELETE FROM board_versions WHERE id IN (
				SELECT id FROM board_versions
				WHERE project_id = ? AND is_automatic = 1
				ORDER BY created_at DESC, id DESC LIMIT -1 OFFSET ?
			)`, projectID, limit)
		if err != nil {
			return fmt.Errorf("errore pulizia cronologia lavagna: %w", err)
		}
	}
	return nil
}

func CreateBoardVersion(db *sql.DB, version BoardVersion) (int64, error) {
	automatic := 0
	if version.IsAutomatic {
		automatic = 1
	}
	result, err := db.Exec(`
		INSERT INTO board_versions (project_id, name, is_automatic, document_json, viewport_json)
		VALUES (?, ?, ?, ?, ?)`, version.ProjectID, version.Name, automatic, version.DocumentJSON, version.ViewportJSON)
	if err != nil {
		return 0, fmt.Errorf("errore creazione versione lavagna: %w", err)
	}
	return result.LastInsertId()
}

func ListBoardVersions(db *sql.DB, projectID int) ([]BoardVersion, error) {
	rows, err := db.Query(`
		SELECT id, project_id, name, is_automatic, created_at
		FROM board_versions WHERE project_id = ?
		ORDER BY created_at DESC, id DESC LIMIT 100`, projectID)
	if err != nil {
		return nil, fmt.Errorf("errore lettura versioni lavagna: %w", err)
	}
	defer rows.Close()
	versions := make([]BoardVersion, 0)
	for rows.Next() {
		var version BoardVersion
		var automatic int
		if err := rows.Scan(&version.ID, &version.ProjectID, &version.Name, &automatic, &version.CreatedAt); err != nil {
			return nil, err
		}
		version.IsAutomatic = automatic == 1
		versions = append(versions, version)
	}
	return versions, rows.Err()
}

func GetBoardVersion(db *sql.DB, projectID int, versionID int64) (*BoardVersion, error) {
	var version BoardVersion
	var automatic int
	err := db.QueryRow(`
		SELECT id, project_id, name, is_automatic, document_json, viewport_json, created_at
		FROM board_versions WHERE id = ? AND project_id = ?`, versionID, projectID).Scan(
		&version.ID, &version.ProjectID, &version.Name, &automatic,
		&version.DocumentJSON, &version.ViewportJSON, &version.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("errore lettura versione lavagna: %w", err)
	}
	version.IsAutomatic = automatic == 1
	return &version, nil
}

func DeleteBoardVersion(db *sql.DB, projectID int, versionID int64) error {
	result, err := db.Exec(`DELETE FROM board_versions WHERE id = ? AND project_id = ?`, versionID, projectID)
	if err != nil {
		return fmt.Errorf("errore eliminazione versione lavagna: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return fmt.Errorf("versione lavagna non trovata")
	}
	return nil
}
