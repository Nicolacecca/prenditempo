package tracker

import (
	"database/sql"
	"fmt"
)

// CreaProgetto crea un nuovo progetto
func CreaProgetto(db *sql.DB, name, description string) (int64, error) {
	insertSQL := `INSERT INTO projects (name, description) VALUES (?, ?)`

	result, err := db.Exec(insertSQL, name, description)
	if err != nil {
		return 0, fmt.Errorf("errore creazione progetto: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("errore ottenimento ID: %v", err)
	}

	fmt.Printf("[DB] Progetto creato: %s (ID: %d)\n", name, id)
	return id, nil
}

// AggiornaProgetto aggiorna nome e descrizione di un progetto esistente
func AggiornaProgetto(db *sql.DB, projectID int, name, description string) error {
	updateSQL := `UPDATE projects SET name = ?, description = ? WHERE id = ?`

	result, err := db.Exec(updateSQL, name, description, projectID)
	if err != nil {
		return fmt.Errorf("errore aggiornamento progetto: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("errore verifica aggiornamento: %v", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("progetto con ID %d non trovato", projectID)
	}

	fmt.Printf("[DB] Progetto ID %d aggiornato: %s\n", projectID, name)
	return nil
}

// AggiornaNotaProgetto aggiorna la nota markdown di un progetto
func AggiornaNotaProgetto(db *sql.DB, projectID int, noteText string) error {
	updateSQL := `UPDATE projects SET note_text = ? WHERE id = ?`

	result, err := db.Exec(updateSQL, noteText, projectID)
	if err != nil {
		return fmt.Errorf("errore aggiornamento nota progetto: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("errore verifica aggiornamento: %v", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("progetto con ID %d non trovato", projectID)
	}

	fmt.Printf("[DB] Nota progetto ID %d aggiornata\n", projectID)
	return nil
}

// AggiornaBoardURLProgetto aggiorna l'URL della lavagna esterna collegata al progetto
func AggiornaBoardURLProgetto(db *sql.DB, projectID int, boardURL string) error {
	updateSQL := `UPDATE projects SET board_url = ? WHERE id = ?`

	result, err := db.Exec(updateSQL, boardURL, projectID)
	if err != nil {
		return fmt.Errorf("errore aggiornamento URL lavagna: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("errore verifica aggiornamento: %v", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("progetto con ID %d non trovato", projectID)
	}

	fmt.Printf("[DB] URL lavagna progetto ID %d aggiornato\n", projectID)
	return nil
}

// MigraNoteLegacy migra le note dalla vecchia tabella notes al campo note_text dei progetti
func MigraNoteLegacy(db *sql.DB) (int, error) {
	// Legge tutte le note vecchie raggruppate per progetto
	query := `
	SELECT n.project_id, n.note_text, n.timestamp, p.name
	FROM notes n
	LEFT JOIN projects p ON n.project_id = p.id
	ORDER BY n.project_id, n.timestamp ASC
	`

	rows, err := db.Query(query)
	if err != nil {
		return 0, fmt.Errorf("errore lettura note legacy: %v", err)
	}
	defer rows.Close()

	// Raggruppa note per progetto
	notesByProject := make(map[int][]string)
	projectNames := make(map[int]string)
	noteCount := 0

	for rows.Next() {
		var projectID int
		var noteText, timestamp string
		var projectName *string

		if err := rows.Scan(&projectID, &noteText, &timestamp, &projectName); err != nil {
			return 0, fmt.Errorf("errore lettura riga nota: %v", err)
		}

		// Formatta la nota con timestamp come header markdown
		formattedNote := fmt.Sprintf("## %s\n\n%s\n", timestamp, noteText)
		notesByProject[projectID] = append(notesByProject[projectID], formattedNote)

		if projectName != nil {
			projectNames[projectID] = *projectName
		}
		noteCount++
	}

	if noteCount == 0 {
		return 0, nil
	}

	// Aggiorna ogni progetto con le sue note concatenate
	migratedProjects := 0
	for projectID, notes := range notesByProject {
		// Carica nota esistente del progetto
		var existingNote string
		err := db.QueryRow(`SELECT COALESCE(note_text, '') FROM projects WHERE id = ?`, projectID).Scan(&existingNote)
		if err != nil {
			fmt.Printf("[DB] Errore lettura nota esistente per progetto %d: %v\n", projectID, err)
			continue
		}

		// Concatena le vecchie note
		combinedNotes := "# Note Importate\n\n"
		for _, note := range notes {
			combinedNotes += note + "\n---\n\n"
		}

		// Se c'era già una nota, la mette in fondo
		if existingNote != "" {
			combinedNotes = existingNote + "\n\n---\n\n" + combinedNotes
		}

		// Aggiorna il progetto
		_, err = db.Exec(`UPDATE projects SET note_text = ? WHERE id = ?`, combinedNotes, projectID)
		if err != nil {
			fmt.Printf("[DB] Errore aggiornamento nota per progetto %d: %v\n", projectID, err)
			continue
		}

		projectName := projectNames[projectID]
		if projectName == "" {
			projectName = fmt.Sprintf("ID %d", projectID)
		}
		fmt.Printf("[DB] Migrate %d note per progetto '%s'\n", len(notes), projectName)
		migratedProjects++
	}

	fmt.Printf("[DB] Migrazione completata: %d note da %d progetti\n", noteCount, migratedProjects)
	return noteCount, nil
}

// CaricaTuttiProgetti carica tutti i progetti
func CaricaTuttiProgetti(db *sql.DB) ([]Project, error) {
	selectSQL := `SELECT id, name, description, created_at, archived, COALESCE(closed_at, ''), COALESCE(note_text, ''), COALESCE(board_url, '') FROM projects ORDER BY created_at DESC`

	rows, err := db.Query(selectSQL)
	if err != nil {
		return nil, fmt.Errorf("errore lettura progetti: %v", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		var archived int
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &archived, &p.ClosedAt, &p.NoteText, &p.BoardURL); err != nil {
			return nil, fmt.Errorf("errore lettura riga: %v", err)
		}
		p.Archived = archived == 1
		projects = append(projects, p)
	}

	return projects, nil
}

// CaricaProgettiAttivi carica solo i progetti attivi (non archiviati) - ottimizzato
func CaricaProgettiAttivi(db *sql.DB) ([]Project, error) {
	selectSQL := `SELECT id, name, description, created_at, archived, COALESCE(closed_at, ''), COALESCE(note_text, ''), COALESCE(board_url, '') FROM projects WHERE archived = 0 ORDER BY created_at DESC`

	rows, err := db.Query(selectSQL)
	if err != nil {
		return nil, fmt.Errorf("errore lettura progetti attivi: %v", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		var archived int
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &archived, &p.ClosedAt, &p.NoteText, &p.BoardURL); err != nil {
			return nil, fmt.Errorf("errore lettura riga: %v", err)
		}
		p.Archived = archived == 1
		projects = append(projects, p)
	}

	return projects, nil
}

// CaricaProgettiArchiviati carica solo i progetti archiviati - ottimizzato
func CaricaProgettiArchiviati(db *sql.DB) ([]Project, error) {
	selectSQL := `SELECT id, name, description, created_at, archived, COALESCE(closed_at, ''), COALESCE(note_text, ''), COALESCE(board_url, '') FROM projects WHERE archived = 1 ORDER BY closed_at DESC`

	rows, err := db.Query(selectSQL)
	if err != nil {
		return nil, fmt.Errorf("errore lettura progetti archiviati: %v", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		var archived int
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &archived, &p.ClosedAt, &p.NoteText, &p.BoardURL); err != nil {
			return nil, fmt.Errorf("errore lettura riga: %v", err)
		}
		p.Archived = archived == 1
		projects = append(projects, p)
	}

	return projects, nil
}

// TrovaProgetto cerca un progetto per nome
func TrovaProgetto(db *sql.DB, name string) (*Project, error) {
	selectSQL := `SELECT id, name, description, created_at, archived, COALESCE(closed_at, ''), COALESCE(note_text, ''), COALESCE(board_url, '') FROM projects WHERE name = ?`

	var p Project
	var archived int
	err := db.QueryRow(selectSQL, name).Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &archived, &p.ClosedAt, &p.NoteText, &p.BoardURL)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("progetto '%s' non trovato", name)
		}
		return nil, fmt.Errorf("errore ricerca progetto: %v", err)
	}
	p.Archived = archived == 1

	return &p, nil
}

// TrovaProgettoById trova un progetto per ID
func TrovaProgettoById(db *sql.DB, id int) (*Project, error) {
	selectSQL := `SELECT id, name, description, created_at, archived, COALESCE(closed_at, ''), COALESCE(note_text, ''), COALESCE(board_url, '') FROM projects WHERE id = ?`

	var p Project
	var archived int
	err := db.QueryRow(selectSQL, id).Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &archived, &p.ClosedAt, &p.NoteText, &p.BoardURL)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("progetto con ID %d non trovato", id)
		}
		return nil, fmt.Errorf("errore ricerca progetto: %v", err)
	}
	p.Archived = archived == 1

	return &p, nil
}

// EliminaProgetto elimina un progetto e tutti i suoi dati associati (sessioni e note)
func EliminaProgetto(db *sql.DB, name string) error {
	// Prima trova l'ID del progetto
	project, err := TrovaProgetto(db, name)
	if err != nil {
		return err
	}

	// Elimina tutte le note associate al progetto
	deleteNotesSQL := `DELETE FROM notes WHERE project_id = ?`
	result, err := db.Exec(deleteNotesSQL, project.ID)
	if err != nil {
		return fmt.Errorf("errore eliminazione note del progetto: %v", err)
	}
	notesDeleted, _ := result.RowsAffected()

	// Elimina tutte le sessioni associate al progetto
	deleteSessionsSQL := `DELETE FROM sessions WHERE project_id = ?`
	result, err = db.Exec(deleteSessionsSQL, project.ID)
	if err != nil {
		return fmt.Errorf("errore eliminazione sessioni del progetto: %v", err)
	}
	sessionsDeleted, _ := result.RowsAffected()

	// Infine, elimina il progetto stesso
	deleteProjectSQL := `DELETE FROM projects WHERE id = ?`
	result, err = db.Exec(deleteProjectSQL, project.ID)
	if err != nil {
		return fmt.Errorf("errore eliminazione progetto: %v", err)
	}

	fmt.Printf("[DB] Progetto eliminato: %s (sessioni: %d, note: %d)\n", name, sessionsDeleted, notesDeleted)
	return nil
}

// EliminaProgettoById elimina un progetto e tutti i dati associati dato l'ID
func EliminaProgettoById(db *sql.DB, projectID int) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("errore avvio eliminazione progetto: %v", err)
	}
	defer tx.Rollback()

	// Recupera il nome del progetto per il log
	var projectName string
	err = tx.QueryRow("SELECT name FROM projects WHERE id = ?", projectID).Scan(&projectName)
	if err != nil {
		return fmt.Errorf("progetto non trovato: %v", err)
	}

	// Rimuovi prima eventuali riferimenti di tracking pendente.
	if _, err := tx.Exec("DELETE FROM pending_tracking WHERE project_id = ?", projectID); err != nil {
		return fmt.Errorf("errore eliminazione tracking pendente: %v", err)
	}

	// Elimina tutte le note associate al progetto
	deleteNotesSQL := `DELETE FROM notes WHERE project_id = ?`
	result, err := tx.Exec(deleteNotesSQL, projectID)
	if err != nil {
		return fmt.Errorf("errore eliminazione note del progetto: %v", err)
	}
	notesDeleted, _ := result.RowsAffected()

	// Elimina tutte le sessioni associate al progetto
	deleteSessionsSQL := `DELETE FROM sessions WHERE project_id = ?`
	result, err = tx.Exec(deleteSessionsSQL, projectID)
	if err != nil {
		return fmt.Errorf("errore eliminazione sessioni del progetto: %v", err)
	}
	sessionsDeleted, _ := result.RowsAffected()

	// Infine, elimina il progetto stesso
	deleteProjectSQL := `DELETE FROM projects WHERE id = ?`
	result, err = tx.Exec(deleteProjectSQL, projectID)
	if err != nil {
		return fmt.Errorf("errore eliminazione progetto: %v", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return fmt.Errorf("progetto %d non trovato", projectID)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("errore conferma eliminazione progetto: %v", err)
	}

	fmt.Printf("[DB] Progetto eliminato: %s (ID: %d, sessioni: %d, note: %d)\n", projectName, projectID, sessionsDeleted, notesDeleted)
	return nil
}

// ArchivaProgetto chiude e archivia un progetto
func ArchivaProgetto(db *sql.DB, projectID int) error {
	updateSQL := `UPDATE projects SET archived = 1, closed_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := db.Exec(updateSQL, projectID)
	if err != nil {
		return fmt.Errorf("errore archiviazione progetto: %v", err)
	}
	fmt.Printf("[DB] Progetto ID %d archiviato\n", projectID)
	return nil
}

// RiattivaProgetto riattiva un progetto archiviato
func RiattivaProgetto(db *sql.DB, projectID int) error {
	updateSQL := `UPDATE projects SET archived = 0, closed_at = NULL WHERE id = ?`
	_, err := db.Exec(updateSQL, projectID)
	if err != nil {
		return fmt.Errorf("errore riattivazione progetto: %v", err)
	}
	fmt.Printf("[DB] Progetto ID %d riattivato\n", projectID)
	return nil
}

// GeneraReportChiusura genera il report di chiusura per un progetto
func GeneraReportChiusura(db *sql.DB, projectID int) (map[string]interface{}, error) {
	// Recupera informazioni progetto
	var project Project
	var archived int
	projectSQL := `SELECT id, name, description, created_at, archived, COALESCE(closed_at, ''), COALESCE(note_text, ''), COALESCE(board_url, '') FROM projects WHERE id = ?`
	err := db.QueryRow(projectSQL, projectID).Scan(&project.ID, &project.Name, &project.Description, &project.CreatedAt, &archived, &project.ClosedAt, &project.NoteText, &project.BoardURL)
	if err != nil {
		return nil, fmt.Errorf("errore lettura progetto: %v", err)
	}
	project.Archived = archived == 1

	// Calcola ore totali
	var totalSeconds int
	totalSQL := `SELECT COALESCE(SUM(seconds), 0) FROM sessions WHERE project_id = ?`
	err = db.QueryRow(totalSQL, projectID).Scan(&totalSeconds)
	if err != nil {
		return nil, fmt.Errorf("errore calcolo ore totali: %v", err)
	}

	// Calcola suddivisione per tipo di attività
	activityBreakdownSQL := `
		SELECT COALESCE(activity_type, 'Nessuna attività'), SUM(seconds)
		FROM sessions
		WHERE project_id = ?
		GROUP BY activity_type
	`
	rows, err := db.Query(activityBreakdownSQL, projectID)
	if err != nil {
		return nil, fmt.Errorf("errore calcolo suddivisione attività: %v", err)
	}
	defer rows.Close()

	breakdown := make(map[string]float64)
	for rows.Next() {
		var activityType string
		var seconds int
		if err := rows.Scan(&activityType, &seconds); err != nil {
			return nil, fmt.Errorf("errore lettura suddivisione: %v", err)
		}
		breakdown[activityType] = float64(seconds) / 3600.0 // Converti in ore
	}

	// Calcola date di inizio e fine
	var startDate, endDate string
	datesSQL := `SELECT COALESCE(MIN(timestamp), ''), COALESCE(MAX(timestamp), '') FROM sessions WHERE project_id = ?`
	err = db.QueryRow(datesSQL, projectID).Scan(&startDate, &endDate)
	if err != nil {
		return nil, fmt.Errorf("errore calcolo date: %v", err)
	}

	report := map[string]interface{}{
		"project_id":          project.ID,
		"project_name":        project.Name,
		"project_description": project.Description,
		"note_text":           project.NoteText,
		"created_at":          project.CreatedAt,
		"closed_at":           project.ClosedAt,
		"start_date":          startDate,
		"end_date":            endDate,
		"total_hours":         float64(totalSeconds) / 3600.0,
		"activity_breakdown":  breakdown,
	}

	return report, nil
}
