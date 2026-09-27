package tracker

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// isDuplicateColumnError verifica se l'errore è dovuto a una colonna già esistente
func isDuplicateColumnError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "duplicate column") ||
		strings.Contains(errStr, "already exists")
}

// InitDB inizializza il database e crea le tabelle
func InitDB(filepath string) (*sql.DB, error) {
	// Apri connessione al database
	db, err := sql.Open("sqlite", filepath)
	if err != nil {
		return nil, fmt.Errorf("errore apertura database: %v", err)
	}
	// Le PRAGMA sono specifiche della connessione. Una sola connessione è
	// sufficiente per questa app desktop e garantisce che siano sempre attive.
	db.SetMaxOpenConns(1)

	// Verifica che la connessione funzioni
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("errore connessione database: %v", err)
	}
	for _, pragma := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA journal_mode = WAL",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("errore configurazione database (%s): %v", pragma, err)
		}
	}

	// Crea la tabella se non esiste
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS applicazioni (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		nome TEXT NOT NULL,
		tempo INTEGER NOT NULL,
		categoria TEXT NOT NULL,
		data_creazione DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err := db.Exec(createTableSQL); err != nil {
		return nil, fmt.Errorf("errore creazione tabella: %v", err)
	}

	// Crea tabella projects
	createProjectsSQL := `
	CREATE TABLE IF NOT EXISTS projects (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		description TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		archived INTEGER DEFAULT 0,
		closed_at DATETIME
	);`

	if _, err := db.Exec(createProjectsSQL); err != nil {
		return nil, fmt.Errorf("errore creazione tabella projects: %v", err)
	}

	// Migrazione: aggiungi colonne archived e closed_at se non esistono
	// Ignoriamo errori "duplicate column" che indicano che la colonna esiste già
	if _, err := db.Exec(`ALTER TABLE projects ADD COLUMN archived INTEGER DEFAULT 0;`); err != nil {
		if !isDuplicateColumnError(err) {
			db.Close()
			return nil, fmt.Errorf("errore migrazione projects.archived: %v", err)
		}
	}
	if _, err := db.Exec(`ALTER TABLE projects ADD COLUMN closed_at DATETIME;`); err != nil {
		if !isDuplicateColumnError(err) {
			db.Close()
			return nil, fmt.Errorf("errore migrazione projects.closed_at: %v", err)
		}
	}
	// Migrazione: aggiungi colonna note_text per nota markdown del progetto
	if _, err := db.Exec(`ALTER TABLE projects ADD COLUMN note_text TEXT DEFAULT '';`); err != nil {
		if !isDuplicateColumnError(err) {
			db.Close()
			return nil, fmt.Errorf("errore migrazione projects.note_text: %v", err)
		}
	}

	// Crea tabella sessions per tracciare sessioni
	createSessionsSQL := `
	CREATE TABLE IF NOT EXISTS sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		app_name TEXT NOT NULL,
		seconds INTEGER NOT NULL,
		project_id INTEGER,
		session_type TEXT DEFAULT 'computer',
		activity_type TEXT DEFAULT NULL,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (project_id) REFERENCES projects(id)
	);`

	if _, err := db.Exec(createSessionsSQL); err != nil {
		return nil, fmt.Errorf("errore creazione tabella sessions: %v", err)
	}

	// Migrazione: aggiungi colonna activity_type se non esiste
	if _, err := db.Exec(`ALTER TABLE sessions ADD COLUMN activity_type TEXT DEFAULT NULL;`); err != nil {
		if !isDuplicateColumnError(err) {
			db.Close()
			return nil, fmt.Errorf("errore migrazione sessions.activity_type: %v", err)
		}
	}

	// Crea tabella notes per le note dei progetti
	createNotesSQL := `
	CREATE TABLE IF NOT EXISTS notes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id INTEGER NOT NULL,
		note_text TEXT NOT NULL,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (project_id) REFERENCES projects(id)
	);`

	if _, err := db.Exec(createNotesSQL); err != nil {
		return nil, fmt.Errorf("errore creazione tabella notes: %v", err)
	}

	createProjectBoardsSQL := `
	CREATE TABLE IF NOT EXISTS project_boards (
		project_id INTEGER PRIMARY KEY,
		schema_version INTEGER NOT NULL DEFAULT 1,
		document_json TEXT NOT NULL DEFAULT '{}',
		viewport_json TEXT NOT NULL DEFAULT '{}',
		legacy_note_imported INTEGER NOT NULL DEFAULT 0,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
	);`

	if _, err := db.Exec(createProjectBoardsSQL); err != nil {
		return nil, fmt.Errorf("errore creazione tabella project_boards: %v", err)
	}

	createBoardAssetsSQL := `
	CREATE TABLE IF NOT EXISTS board_assets (
		id TEXT PRIMARY KEY,
		project_id INTEGER NOT NULL,
		file_name TEXT NOT NULL,
		mime_type TEXT NOT NULL,
		data BLOB NOT NULL,
		size INTEGER NOT NULL,
		sha256 TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
		UNIQUE (project_id, sha256)
	);`

	if _, err := db.Exec(createBoardAssetsSQL); err != nil {
		return nil, fmt.Errorf("errore creazione tabella board_assets: %v", err)
	}

	createBoardVersionsSQL := `
	CREATE TABLE IF NOT EXISTS board_versions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		is_automatic INTEGER NOT NULL DEFAULT 0,
		document_json TEXT NOT NULL,
		viewport_json TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
	);`

	if _, err := db.Exec(createBoardVersionsSQL); err != nil {
		return nil, fmt.Errorf("errore creazione tabella board_versions: %v", err)
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_board_versions_project_date ON board_versions(project_id, created_at DESC, id DESC)`); err != nil {
		return nil, fmt.Errorf("errore creazione indice board_versions: %v", err)
	}

	// Crea tabella activity_types per i tipi di attività configurabili
	createActivityTypesSQL := `
	CREATE TABLE IF NOT EXISTS activity_types (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		color_variant REAL DEFAULT 0.0,
		display_order INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err := db.Exec(createActivityTypesSQL); err != nil {
		return nil, fmt.Errorf("errore creazione tabella activity_types: %v", err)
	}

	// Migrazione: aggiungi colonna pattern se non esiste
	if _, err := db.Exec(`ALTER TABLE activity_types ADD COLUMN pattern TEXT DEFAULT 'solid';`); err != nil {
		if !isDuplicateColumnError(err) {
			db.Close()
			return nil, fmt.Errorf("errore migrazione activity_types.pattern: %v", err)
		}
	}

	// Inserisci tipi di attività di default se la tabella è vuota
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM activity_types").Scan(&count); err != nil {
		return nil, fmt.Errorf("errore conteggio tipi attività: %v", err)
	}
	if count == 0 {
		defaultTypes := []struct {
			name         string
			colorVariant float64
			pattern      string
			order        int
		}{
			{"RICERCA", 0.3, "dots", 1},
			{"PROGETTAZIONE", 0.0, "solid", 2},
			{"REALIZZAZIONE", -0.3, "stripes", 3},
		}

		for _, t := range defaultTypes {
			if _, err := db.Exec("INSERT INTO activity_types (name, color_variant, pattern, display_order) VALUES (?, ?, ?, ?)",
				t.name, t.colorVariant, t.pattern, t.order); err != nil {
				return nil, fmt.Errorf("errore inserimento tipo attività predefinito: %v", err)
			}
		}
		fmt.Println("[DB] Tipi di attività predefiniti inseriti")
	}

	// Crea tabella settings per le impostazioni dell'applicazione
	createSettingsSQL := `
	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);`

	if _, err := db.Exec(createSettingsSQL); err != nil {
		return nil, fmt.Errorf("errore creazione tabella settings: %v", err)
	}

	// Inserisci impostazioni di default se non esistono
	if _, err := db.Exec("INSERT OR IGNORE INTO settings (key, value) VALUES ('auto_start', 'false')"); err != nil {
		return nil, fmt.Errorf("errore impostazioni predefinite: %v", err)
	}

	// Crea indici per migliorare le performance delle query
	for _, statement := range []string{
		`CREATE INDEX IF NOT EXISTS idx_sessions_timestamp ON sessions(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_project_id ON sessions(project_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_app_name ON sessions(app_name)`,
		`CREATE INDEX IF NOT EXISTS idx_notes_project_id ON notes(project_id)`,
		`CREATE INDEX IF NOT EXISTS idx_notes_timestamp ON notes(timestamp)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			return nil, fmt.Errorf("errore creazione indice: %v", err)
		}
	}

	// Crea tabella day_events per feste, ferie e permessi
	createDayEventsSQL := `
	CREATE TABLE IF NOT EXISTS day_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		date TEXT NOT NULL UNIQUE,
		event_type TEXT NOT NULL,
		hours REAL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := db.Exec(createDayEventsSQL); err != nil {
		return nil, fmt.Errorf("errore creazione tabella day_events: %v", err)
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_day_events_date ON day_events(date)`); err != nil {
		return nil, fmt.Errorf("errore indice eventi: %v", err)
	}

	// Crea tabella pending_tracking per tracciare sessioni in corso (auto-save)
	createPendingTrackingSQL := `
	CREATE TABLE IF NOT EXISTS pending_tracking (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id INTEGER NOT NULL,
		project_id INTEGER,
		activity_type TEXT,
		start_time DATETIME NOT NULL,
		last_saved_seconds INTEGER DEFAULT 0,
		last_update DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (session_id) REFERENCES sessions(id),
		FOREIGN KEY (project_id) REFERENCES projects(id)
	);`

	if _, err := db.Exec(createPendingTrackingSQL); err != nil {
		return nil, fmt.Errorf("errore creazione tabella pending_tracking: %v", err)
	}

	// I trigger su day_events vengono ricreati ad ogni avvio (drop + create) per
	// permettere di aggiornare l'elenco di event_type ammessi (es. aggiunta di
	// 'malattia') anche sui database esistenti: "CREATE TRIGGER IF NOT EXISTS"
	// da solo non aggiornerebbe mai un trigger già presente.
	for _, statement := range []string{
		`DROP TRIGGER IF EXISTS validate_day_events_insert`,
		`DROP TRIGGER IF EXISTS validate_day_events_update`,
	} {
		if _, err := db.Exec(statement); err != nil {
			db.Close()
			return nil, fmt.Errorf("errore aggiornamento vincoli database: %v", err)
		}
	}

	// Le API applicative validano i dati; i trigger proteggono anche scritture
	// dirette o future migrazioni che potrebbero aggirare tali API.
	for _, statement := range []string{
		`CREATE TRIGGER IF NOT EXISTS validate_sessions_insert BEFORE INSERT ON sessions
		 WHEN NEW.seconds < 0 OR trim(NEW.app_name) = '' OR NEW.session_type NOT IN ('computer','off-computer','manual')
		 BEGIN SELECT RAISE(ABORT, 'sessione non valida'); END`,
		`CREATE TRIGGER IF NOT EXISTS validate_sessions_update BEFORE UPDATE ON sessions
		 WHEN NEW.seconds < 0 OR trim(NEW.app_name) = '' OR NEW.session_type NOT IN ('computer','off-computer','manual')
		 BEGIN SELECT RAISE(ABORT, 'sessione non valida'); END`,
		`CREATE TRIGGER validate_day_events_insert BEFORE INSERT ON day_events
		 WHEN NEW.event_type NOT IN ('holiday','vacation','permesso','malattia','trasferta') OR NEW.hours < 0 OR NEW.hours > 24
		   OR (NEW.event_type != 'permesso' AND NEW.hours != 0)
		 BEGIN SELECT RAISE(ABORT, 'evento non valido'); END`,
		`CREATE TRIGGER validate_day_events_update BEFORE UPDATE ON day_events
		 WHEN NEW.event_type NOT IN ('holiday','vacation','permesso','malattia','trasferta') OR NEW.hours < 0 OR NEW.hours > 24
		   OR (NEW.event_type != 'permesso' AND NEW.hours != 0)
		 BEGIN SELECT RAISE(ABORT, 'evento non valido'); END`,
	} {
		if _, err := db.Exec(statement); err != nil {
			db.Close()
			return nil, fmt.Errorf("errore creazione vincoli database: %v", err)
		}
	}

	fmt.Println("[DB] Database inizializzato con successo")
	return db, nil

}

// SalvaSessioneConTipo salva una sessione specificando il tipo, activity_type e timestamp opzionale
func SalvaSessioneConTipo(db *sql.DB, appName string, seconds int, projectID *int, sessionType string, activityType *string, timestamp string) error {
	var insertSQL string
	var err error

	if timestamp != "" {
		// Salva con timestamp specifico
		insertSQL = `INSERT INTO sessions (app_name, seconds, project_id, session_type, activity_type, timestamp) VALUES (?, ?, ?, ?, ?, ?)`
		_, err = db.Exec(insertSQL, appName, seconds, projectID, sessionType, activityType, timestamp)
	} else {
		// Usa timestamp corrente del sistema Go (già in ora locale)
		now := time.Now().Format("2006-01-02 15:04:05")
		insertSQL = `INSERT INTO sessions (app_name, seconds, project_id, session_type, activity_type, timestamp) VALUES (?, ?, ?, ?, ?, ?)`
		_, err = db.Exec(insertSQL, appName, seconds, projectID, sessionType, activityType, now)
	}

	if err != nil {
		return fmt.Errorf("errore salvataggio sessione: %v", err)
	}

	return nil
}

// === FUNZIONI PER PROGETTI ===

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
	selectSQL := `SELECT id, name, description, created_at, archived, COALESCE(closed_at, ''), COALESCE(note_text, '') FROM projects ORDER BY created_at DESC`

	rows, err := db.Query(selectSQL)
	if err != nil {
		return nil, fmt.Errorf("errore lettura progetti: %v", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		var archived int
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &archived, &p.ClosedAt, &p.NoteText); err != nil {
			return nil, fmt.Errorf("errore lettura riga: %v", err)
		}
		p.Archived = archived == 1
		projects = append(projects, p)
	}

	return projects, nil
}

// CaricaProgettiAttivi carica solo i progetti attivi (non archiviati) - ottimizzato
func CaricaProgettiAttivi(db *sql.DB) ([]Project, error) {
	selectSQL := `SELECT id, name, description, created_at, archived, COALESCE(closed_at, ''), COALESCE(note_text, '') FROM projects WHERE archived = 0 ORDER BY created_at DESC`

	rows, err := db.Query(selectSQL)
	if err != nil {
		return nil, fmt.Errorf("errore lettura progetti attivi: %v", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		var archived int
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &archived, &p.ClosedAt, &p.NoteText); err != nil {
			return nil, fmt.Errorf("errore lettura riga: %v", err)
		}
		p.Archived = archived == 1
		projects = append(projects, p)
	}

	return projects, nil
}

// CaricaProgettiArchiviati carica solo i progetti archiviati - ottimizzato
func CaricaProgettiArchiviati(db *sql.DB) ([]Project, error) {
	selectSQL := `SELECT id, name, description, created_at, archived, COALESCE(closed_at, ''), COALESCE(note_text, '') FROM projects WHERE archived = 1 ORDER BY closed_at DESC`

	rows, err := db.Query(selectSQL)
	if err != nil {
		return nil, fmt.Errorf("errore lettura progetti archiviati: %v", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		var archived int
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &archived, &p.ClosedAt, &p.NoteText); err != nil {
			return nil, fmt.Errorf("errore lettura riga: %v", err)
		}
		p.Archived = archived == 1
		projects = append(projects, p)
	}

	return projects, nil
}

// TrovaProgetto cerca un progetto per nome
func TrovaProgetto(db *sql.DB, name string) (*Project, error) {
	selectSQL := `SELECT id, name, description, created_at, archived, COALESCE(closed_at, ''), COALESCE(note_text, '') FROM projects WHERE name = ?`

	var p Project
	var archived int
	err := db.QueryRow(selectSQL, name).Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &archived, &p.ClosedAt, &p.NoteText)
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
	selectSQL := `SELECT id, name, description, created_at, archived, COALESCE(closed_at, ''), COALESCE(note_text, '') FROM projects WHERE id = ?`

	var p Project
	var archived int
	err := db.QueryRow(selectSQL, id).Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &archived, &p.ClosedAt, &p.NoteText)
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

// SessionDetail rappresenta una sessione dettagliata con timestamp
type SessionDetail struct {
	ID           int
	AppName      string
	Seconds      int
	ProjectID    *int
	ProjectName  string
	SessionType  string
	ActivityType *string
	Timestamp    string
}

// CaricaSessioniDettagliate carica le sessioni con timestamp per la timeline
// Ottimizzato: usa confronto diretto invece di DATE() per sfruttare gli indici
func CaricaSessioniDettagliate(db *sql.DB, startDate, endDate string) ([]SessionDetail, error) {
	// Costruisci range di date per query ottimizzata (usa indice su timestamp)
	startDateTime := startDate + " 00:00:00"
	endDateTime := endDate + " 23:59:59"

	query := `
	SELECT
		s.id,
		s.app_name,
		s.seconds,
		s.project_id,
		COALESCE(p.name, 'Nessun progetto') as project_name,
		COALESCE(s.session_type, 'computer') as session_type,
		s.activity_type,
		s.timestamp
	FROM sessions s
	LEFT JOIN projects p ON s.project_id = p.id
	WHERE s.timestamp >= ? AND s.timestamp <= ?
	ORDER BY s.timestamp ASC
	`

	rows, err := db.Query(query, startDateTime, endDateTime)
	if err != nil {
		return nil, fmt.Errorf("errore query sessioni dettagliate: %v", err)
	}
	defer rows.Close()

	var sessions []SessionDetail
	for rows.Next() {
		var s SessionDetail
		if err := rows.Scan(&s.ID, &s.AppName, &s.Seconds, &s.ProjectID, &s.ProjectName, &s.SessionType, &s.ActivityType, &s.Timestamp); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}

	return sessions, nil
}

// CaricaSessioniDettagliatePerProgetto carica le sessioni dettagliate di un singolo
// progetto, filtrando lato SQL invece di caricare l'intero storico in memoria.
func CaricaSessioniDettagliatePerProgetto(db *sql.DB, projectID int) ([]SessionDetail, error) {
	query := `
	SELECT
		s.id,
		s.app_name,
		s.seconds,
		s.project_id,
		COALESCE(p.name, 'Nessun progetto') as project_name,
		COALESCE(s.session_type, 'computer') as session_type,
		s.activity_type,
		s.timestamp
	FROM sessions s
	LEFT JOIN projects p ON s.project_id = p.id
	WHERE s.project_id = ?
	ORDER BY s.timestamp ASC
	`

	rows, err := db.Query(query, projectID)
	if err != nil {
		return nil, fmt.Errorf("errore query sessioni progetto: %v", err)
	}
	defer rows.Close()

	var sessions []SessionDetail
	for rows.Next() {
		var s SessionDetail
		if err := rows.Scan(&s.ID, &s.AppName, &s.Seconds, &s.ProjectID, &s.ProjectName, &s.SessionType, &s.ActivityType, &s.Timestamp); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}

	return sessions, nil
}

// AggiornaActivityType aggiorna il tipo di attività di una sessione
func AggiornaActivityType(db *sql.DB, sessionID int, activityType *string) error {
	updateSQL := `UPDATE sessions SET activity_type = ? WHERE id = ?`

	_, err := db.Exec(updateSQL, activityType, sessionID)
	if err != nil {
		return fmt.Errorf("errore aggiornamento activity_type: %v", err)
	}

	fmt.Printf("[DB] Activity type aggiornato per sessione ID %d\n", sessionID)
	return nil
}

// === FUNZIONI PER ARCHIVIAZIONE PROGETTI ===

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
	projectSQL := `SELECT id, name, description, created_at, archived, COALESCE(closed_at, ''), COALESCE(note_text, '') FROM projects WHERE id = ?`
	err := db.QueryRow(projectSQL, projectID).Scan(&project.ID, &project.Name, &project.Description, &project.CreatedAt, &archived, &project.ClosedAt, &project.NoteText)
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

// === FUNZIONI PER MODIFICA SESSIONI ===

// EliminaSessione elimina una sessione dal database
func EliminaSessione(db *sql.DB, sessionID int) error {
	deleteSQL := `DELETE FROM sessions WHERE id = ?`

	result, err := db.Exec(deleteSQL, sessionID)
	if err != nil {
		return fmt.Errorf("errore eliminazione sessione: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("errore verifica eliminazione: %v", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("sessione non trovata")
	}

	fmt.Printf("[DB] Sessione ID %d eliminata\n", sessionID)
	return nil
}

// AggiornaDurataSessione aggiorna la durata di una sessione
func AggiornaDurataSessione(db *sql.DB, sessionID int, nuoviSecondi int) error {
	updateSQL := `UPDATE sessions SET seconds = ? WHERE id = ?`

	result, err := db.Exec(updateSQL, nuoviSecondi, sessionID)
	if err != nil {
		return fmt.Errorf("errore aggiornamento durata: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("errore verifica aggiornamento: %v", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("sessione non trovata")
	}

	fmt.Printf("[DB] Durata sessione ID %d aggiornata a %d secondi\n", sessionID, nuoviSecondi)
	return nil
}

// AggiornaSessioneCompleta aggiorna timestamp, durata e tipo attività di una sessione
func AggiornaSessioneCompleta(db *sql.DB, sessionID int, newTimestamp string, newSeconds int, activityType *string) error {
	updateSQL := `UPDATE sessions SET timestamp = ?, seconds = ?, activity_type = ? WHERE id = ?`

	result, err := db.Exec(updateSQL, newTimestamp, newSeconds, activityType, sessionID)
	if err != nil {
		return fmt.Errorf("errore aggiornamento sessione: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("errore verifica aggiornamento: %v", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("sessione non trovata")
	}

	fmt.Printf("[DB] Sessione ID %d aggiornata: timestamp=%s, secondi=%d\n", sessionID, newTimestamp, newSeconds)
	return nil
}

// CaricaSessioneById carica una singola sessione dato l'ID
func CaricaSessioneById(db *sql.DB, sessionID int) (*SessionDetail, error) {
	query := `
	SELECT
		s.id,
		s.app_name,
		s.seconds,
		s.project_id,
		COALESCE(p.name, 'Nessun progetto') as project_name,
		COALESCE(s.session_type, 'computer') as session_type,
		s.activity_type,
		s.timestamp
	FROM sessions s
	LEFT JOIN projects p ON s.project_id = p.id
	WHERE s.id = ?
	`

	var s SessionDetail
	err := db.QueryRow(query, sessionID).Scan(&s.ID, &s.AppName, &s.Seconds, &s.ProjectID, &s.ProjectName, &s.SessionType, &s.ActivityType, &s.Timestamp)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("sessione con ID %d non trovata", sessionID)
		}
		return nil, fmt.Errorf("errore caricamento sessione: %v", err)
	}

	return &s, nil
}

// DividiSessione divide una sessione in due parti
func DividiSessione(db *sql.DB, sessionID int, secondiPrimaParte int, activityTypePrimaParte *string, activityTypeSecondaParte *string) error {
	if secondiPrimaParte <= 0 {
		return fmt.Errorf("la prima parte deve avere durata positiva")
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("errore avvio transazione divisione: %v", err)
	}
	defer tx.Rollback()

	// Prima carica la sessione originale
	var appName, sessionType, timestamp string
	var seconds int
	var projectID *int
	var activityType *string

	query := `SELECT app_name, seconds, project_id, session_type, activity_type, timestamp FROM sessions WHERE id = ?`
	err = tx.QueryRow(query, sessionID).Scan(&appName, &seconds, &projectID, &sessionType, &activityType, &timestamp)
	if err != nil {
		return fmt.Errorf("errore caricamento sessione: %v", err)
	}

	if secondiPrimaParte >= seconds {
		return fmt.Errorf("la prima parte deve essere minore della durata totale")
	}

	secondiSecondaParte := seconds - secondiPrimaParte

	// Aggiorna la sessione originale con la prima parte
	updateSQL := `UPDATE sessions SET seconds = ?, activity_type = ? WHERE id = ?`
	_, err = tx.Exec(updateSQL, secondiPrimaParte, activityTypePrimaParte, sessionID)
	if err != nil {
		return fmt.Errorf("errore aggiornamento prima parte: %v", err)
	}

	// Calcola il timestamp per la seconda parte (timestamp originale + secondi prima parte)
	timestampOriginale, err := parseTimestamp(timestamp)
	if err != nil {
		return fmt.Errorf("errore parsing timestamp: %v", err)
	}

	timestampSecondaParte := timestampOriginale.Add(time.Duration(secondiPrimaParte) * time.Second)
	timestampSecondaParteStr := timestampSecondaParte.Format("2006-01-02 15:04:05")

	// Crea una nuova sessione per la seconda parte
	insertSQL := `INSERT INTO sessions (app_name, seconds, project_id, session_type, activity_type, timestamp) VALUES (?, ?, ?, ?, ?, ?)`
	_, err = tx.Exec(insertSQL, appName, secondiSecondaParte, projectID, sessionType, activityTypeSecondaParte, timestampSecondaParteStr)
	if err != nil {
		return fmt.Errorf("errore creazione seconda parte: %v", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("errore conferma divisione sessione: %v", err)
	}
	fmt.Printf("[DB] Sessione ID %d divisa in due parti: %d sec e %d sec\n", sessionID, secondiPrimaParte, secondiSecondaParte)
	return nil
}

// parseTimestamp converte un timestamp string in time.Time
func parseTimestamp(timestamp string) (time.Time, error) {
	// Prova diversi formati
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
	}

	for _, format := range formats {
		t, err := time.Parse(format, timestamp)
		if err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("formato timestamp non riconosciuto: %s", timestamp)
}

// CreaSessione crea una nuova sessione manuale
func CreaSessione(db *sql.DB, appName string, seconds int, projectID *int, sessionType string, activityType *string, timestamp string) error {
	insertSQL := `INSERT INTO sessions (app_name, seconds, project_id, session_type, activity_type, timestamp) VALUES (?, ?, ?, ?, ?, ?)`

	result, err := db.Exec(insertSQL, appName, seconds, projectID, sessionType, activityType, timestamp)
	if err != nil {
		return fmt.Errorf("errore creazione sessione: %v", err)
	}

	sessionID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("errore recupero ID sessione: %v", err)
	}

	fmt.Printf("[DB] Nuova sessione creata con ID %d: %s (%d sec) alle %s\n", sessionID, appName, seconds, timestamp)
	return nil
}

// === FUNZIONI PER TIPI DI ATTIVITÀ ===

// ActivityType rappresenta un tipo di attività configurabile
type ActivityType struct {
	ID           int
	Name         string
	ColorVariant float64
	Pattern      string
	DisplayOrder int
	CreatedAt    string
}

// CaricaTipiAttivita carica tutti i tipi di attività
func CaricaTipiAttivita(db *sql.DB) ([]ActivityType, error) {
	query := `SELECT id, name, color_variant, COALESCE(pattern, 'solid'), display_order, COALESCE(created_at, '') FROM activity_types ORDER BY display_order ASC`

	rows, err := db.Query(query)
	if err != nil {
		fmt.Printf("[DB] Errore query tipi attività: %v\n", err)
		return nil, fmt.Errorf("errore query tipi attività: %v", err)
	}
	defer rows.Close()

	var types []ActivityType
	for rows.Next() {
		var t ActivityType
		if err := rows.Scan(&t.ID, &t.Name, &t.ColorVariant, &t.Pattern, &t.DisplayOrder, &t.CreatedAt); err != nil {
			fmt.Printf("[DB] Errore scan tipo attività: %v\n", err)
			return nil, err
		}
		types = append(types, t)
	}

	fmt.Printf("[DB] Caricati %d tipi di attività\n", len(types))
	return types, nil
}

// CreaTipoAttivita crea un nuovo tipo di attività
func CreaTipoAttivita(db *sql.DB, name string, colorVariant float64, pattern string, displayOrder int) (int64, error) {
	fmt.Printf("[DB] Tentativo creazione tipo attività: name=%s, color=%.2f, pattern=%s, order=%d\n", name, colorVariant, pattern, displayOrder)

	insertSQL := `INSERT INTO activity_types (name, color_variant, pattern, display_order) VALUES (?, ?, ?, ?)`

	result, err := db.Exec(insertSQL, name, colorVariant, pattern, displayOrder)
	if err != nil {
		fmt.Printf("[DB] ERRORE creazione tipo attività: %v\n", err)
		return 0, fmt.Errorf("errore creazione tipo attività: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		fmt.Printf("[DB] ERRORE recupero ID: %v\n", err)
		return 0, fmt.Errorf("errore recupero ID: %v", err)
	}

	fmt.Printf("[DB] Tipo attività creato con successo: %s (ID %d)\n", name, id)
	return id, nil
}

// AggiornaTipoAttivita aggiorna un tipo di attività esistente
func AggiornaTipoAttivita(db *sql.DB, id int, name string, colorVariant float64, pattern string, displayOrder int) error {
	updateSQL := `UPDATE activity_types SET name = ?, color_variant = ?, pattern = ?, display_order = ? WHERE id = ?`

	result, err := db.Exec(updateSQL, name, colorVariant, pattern, displayOrder, id)
	if err != nil {
		return fmt.Errorf("errore aggiornamento tipo attività: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("errore verifica aggiornamento: %v", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("tipo attività non trovato")
	}

	fmt.Printf("[DB] Tipo attività ID %d aggiornato\n", id)
	return nil
}

// EliminaTipoAttivita elimina un tipo di attività
func EliminaTipoAttivita(db *sql.DB, id int) error {
	// Prima aggiorna tutte le sessioni che usano questo tipo a NULL
	updateSessionsSQL := `UPDATE sessions SET activity_type = NULL WHERE activity_type = (SELECT name FROM activity_types WHERE id = ?)`
	_, err := db.Exec(updateSessionsSQL, id)
	if err != nil {
		return fmt.Errorf("errore aggiornamento sessioni: %v", err)
	}

	// Poi elimina il tipo
	deleteSQL := `DELETE FROM activity_types WHERE id = ?`
	result, err := db.Exec(deleteSQL, id)
	if err != nil {
		return fmt.Errorf("errore eliminazione tipo attività: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("errore verifica eliminazione: %v", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("tipo attività non trovato")
	}

	fmt.Printf("[DB] Tipo attività ID %d eliminato\n", id)
	return nil
}

// AggiornaOrdineTipoAttivita aggiorna l'ordine di visualizzazione di un tipo attività
func AggiornaOrdineTipoAttivita(db *sql.DB, id int, displayOrder int) error {
	updateSQL := `UPDATE activity_types SET display_order = ? WHERE id = ?`
	_, err := db.Exec(updateSQL, displayOrder, id)
	if err != nil {
		return fmt.Errorf("errore aggiornamento ordine tipo attività: %v", err)
	}
	fmt.Printf("[DB] Ordine tipo attività ID %d aggiornato a %d\n", id, displayOrder)
	return nil
}

// === SETTINGS ===

// GetSetting legge un'impostazione dal database
func GetSetting(db *sql.DB, key string) (string, error) {
	var value string
	query := `SELECT value FROM settings WHERE key = ?`
	err := db.QueryRow(query, key).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil // Chiave non trovata, restituisci stringa vuota
		}
		return "", fmt.Errorf("errore lettura impostazione: %v", err)
	}
	return value, nil
}

// SetSetting salva o aggiorna un'impostazione nel database
func SetSetting(db *sql.DB, key, value string) error {
	insertSQL := `INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)`
	_, err := db.Exec(insertSQL, key, value)
	if err != nil {
		return fmt.Errorf("errore salvataggio impostazione: %v", err)
	}
	fmt.Printf("[DB] Impostazione '%s' = '%s' salvata\n", key, value)
	return nil
}

// === FUNZIONI PER PENDING TRACKING (AUTO-SAVE) ===

// PendingTracking rappresenta una sessione di tracking in corso
type PendingTracking struct {
	ID               int
	SessionID        int
	ProjectID        *int
	ActivityType     *string
	StartTime        string
	LastSavedSeconds int
	LastUpdate       string
}

// StartPendingTracking crea una nuova sessione e registra il tracking pendente
func StartPendingTracking(db *sql.DB, projectID *int, activityType *string, startTime string) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("errore avvio transazione tracking: %v", err)
	}
	defer tx.Rollback()
	var pendingCount int
	if err := tx.QueryRow("SELECT COUNT(*) FROM pending_tracking").Scan(&pendingCount); err != nil {
		return 0, fmt.Errorf("errore verifica tracking pendenti: %v", err)
	}
	if pendingCount > 0 {
		return 0, fmt.Errorf("esiste già un tracking pendente")
	}

	// Crea la sessione con 0 secondi (verrà aggiornata periodicamente)
	insertSessionSQL := `INSERT INTO sessions (app_name, seconds, project_id, session_type, activity_type, timestamp) VALUES (?, ?, ?, ?, ?, ?)`
	result, err := tx.Exec(insertSessionSQL, "Sessione di lavoro", 0, projectID, "computer", activityType, startTime)
	if err != nil {
		return 0, fmt.Errorf("errore creazione sessione pendente: %v", err)
	}

	sessionID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("errore recupero ID sessione: %v", err)
	}

	// Registra il pending tracking
	insertPendingSQL := `INSERT INTO pending_tracking (session_id, project_id, activity_type, start_time, last_saved_seconds) VALUES (?, ?, ?, ?, ?)`
	_, err = tx.Exec(insertPendingSQL, sessionID, projectID, activityType, startTime, 0)
	if err != nil {
		return 0, fmt.Errorf("errore registrazione pending tracking: %v", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("errore conferma tracking pendente: %v", err)
	}

	fmt.Printf("[DB] Pending tracking avviato - Session ID: %d\n", sessionID)
	return sessionID, nil
}

// UpdatePendingTracking aggiorna i secondi della sessione pendente
func UpdatePendingTracking(db *sql.DB, sessionID int64, totalSeconds int) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("errore avvio aggiornamento tracking: %v", err)
	}
	defer tx.Rollback()

	// Aggiorna i secondi nella sessione
	updateSessionSQL := `UPDATE sessions SET seconds = ? WHERE id = ?`
	result, err := tx.Exec(updateSessionSQL, totalSeconds, sessionID)
	if err != nil {
		return fmt.Errorf("errore aggiornamento sessione: %v", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return fmt.Errorf("sessione pendente %d non trovata", sessionID)
	}

	// Aggiorna last_saved_seconds e last_update nel pending tracking
	updatePendingSQL := `UPDATE pending_tracking SET last_saved_seconds = ?, last_update = CURRENT_TIMESTAMP WHERE session_id = ?`
	result, err = tx.Exec(updatePendingSQL, totalSeconds, sessionID)
	if err != nil {
		return fmt.Errorf("errore aggiornamento pending tracking: %v", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return fmt.Errorf("tracking pendente %d non trovato", sessionID)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("errore conferma aggiornamento tracking: %v", err)
	}

	fmt.Printf("[DB] Pending tracking aggiornato - Session ID: %d, Secondi: %d\n", sessionID, totalSeconds)
	return nil
}

// FinalizePendingTracking finalizza la sessione e rimuove il pending tracking
func FinalizePendingTracking(db *sql.DB, sessionID int64, finalSeconds int) error {
	if finalSeconds < 0 {
		return fmt.Errorf("durata finale non valida")
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("errore avvio finalizzazione tracking: %v", err)
	}
	defer tx.Rollback()

	if finalSeconds == 0 {
		if _, err := tx.Exec(`DELETE FROM pending_tracking WHERE session_id = ?`, sessionID); err != nil {
			return fmt.Errorf("errore rimozione tracking vuoto: %v", err)
		}
		if _, err := tx.Exec(`DELETE FROM sessions WHERE id = ?`, sessionID); err != nil {
			return fmt.Errorf("errore rimozione sessione vuota: %v", err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("errore conferma rimozione tracking vuoto: %v", err)
		}
		return nil
	}

	// Aggiorna i secondi finali nella sessione (timestamp = start_time, invariato)
	updateSessionSQL := `UPDATE sessions SET seconds = ? WHERE id = ?`
	result, err := tx.Exec(updateSessionSQL, finalSeconds, sessionID)
	if err != nil {
		return fmt.Errorf("errore finalizzazione sessione: %v", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return fmt.Errorf("sessione pendente %d non trovata", sessionID)
	}

	// Rimuovi il pending tracking
	deletePendingSQL := `DELETE FROM pending_tracking WHERE session_id = ?`
	result, err = tx.Exec(deletePendingSQL, sessionID)
	if err != nil {
		return fmt.Errorf("errore rimozione pending tracking: %v", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return fmt.Errorf("tracking pendente %d non trovato", sessionID)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("errore conferma finalizzazione tracking: %v", err)
	}

	fmt.Printf("[DB] Pending tracking finalizzato - Session ID: %d, Secondi finali: %d\n", sessionID, finalSeconds)
	return nil
}

// GetAllPendingTracking restituisce tutte le sessioni pendenti (per recovery all'avvio)
func GetAllPendingTracking(db *sql.DB) ([]PendingTracking, error) {
	query := `SELECT id, session_id, project_id, activity_type, start_time, last_saved_seconds, last_update FROM pending_tracking`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("errore query pending tracking: %v", err)
	}
	defer rows.Close()

	var pendingList []PendingTracking
	for rows.Next() {
		var p PendingTracking
		if err := rows.Scan(&p.ID, &p.SessionID, &p.ProjectID, &p.ActivityType, &p.StartTime, &p.LastSavedSeconds, &p.LastUpdate); err != nil {
			return nil, err
		}
		pendingList = append(pendingList, p)
	}

	return pendingList, nil
}

// RecoverPendingTracking recupera e finalizza sessioni pendenti da crash precedenti
func RecoverPendingTracking(db *sql.DB) (int, error) {
	pendingList, err := GetAllPendingTracking(db)
	if err != nil {
		return 0, err
	}

	recovered := 0
	for _, p := range pendingList {
		// Finalizza la sessione con i secondi salvati
		if p.LastSavedSeconds > 0 {
			// Aggiorna la sessione con i secondi salvati (timestamp = start_time, invariato)
			updateSQL := `UPDATE sessions SET seconds = ? WHERE id = ?`
			_, err := db.Exec(updateSQL, p.LastSavedSeconds, p.SessionID)
			if err != nil {
				fmt.Printf("[DB] Errore recupero sessione ID %d: %v\n", p.SessionID, err)
				continue
			}
			fmt.Printf("[DB] Sessione ID %d recuperata con %d secondi\n", p.SessionID, p.LastSavedSeconds)
		} else {
			// Se 0 secondi, elimina la sessione vuota
			deleteSQL := `DELETE FROM sessions WHERE id = ?`
			_, err := db.Exec(deleteSQL, p.SessionID)
			if err != nil {
				fmt.Printf("[DB] Errore eliminazione sessione vuota ID %d: %v\n", p.SessionID, err)
			} else {
				fmt.Printf("[DB] Sessione vuota ID %d eliminata\n", p.SessionID)
			}
		}

		// Rimuovi il pending tracking
		deletePendingSQL := `DELETE FROM pending_tracking WHERE id = ?`
		_, err := db.Exec(deletePendingSQL, p.ID)
		if err != nil {
			fmt.Printf("[DB] Errore rimozione pending tracking ID %d: %v\n", p.ID, err)
			continue
		}

		recovered++
	}

	if recovered > 0 {
		fmt.Printf("[DB] Recuperate %d sessioni pendenti\n", recovered)
	}

	return recovered, nil
}
