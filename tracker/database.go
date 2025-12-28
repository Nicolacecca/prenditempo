package tracker

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// InitDB inizializza il database e crea le tabelle
func InitDB(filepath string) (*sql.DB, error) {
	// Apri connessione al database
	db, err := sql.Open("sqlite", filepath)
	if err != nil {
		return nil, fmt.Errorf("errore apertura database: %v", err)
	}

	// Verifica che la connessione funzioni
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("errore connessione database: %v", err)
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
	db.Exec(`ALTER TABLE projects ADD COLUMN archived INTEGER DEFAULT 0;`)
	db.Exec(`ALTER TABLE projects ADD COLUMN closed_at DATETIME;`)

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
	alterSQL := `
	ALTER TABLE sessions ADD COLUMN activity_type TEXT DEFAULT NULL;
	`
	// Ignora l'errore se la colonna esiste già
	db.Exec(alterSQL)

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
	alterPatternSQL := `ALTER TABLE activity_types ADD COLUMN pattern TEXT DEFAULT 'solid';`
	db.Exec(alterPatternSQL) // Ignora errore se esiste già

	// Inserisci tipi di attività di default se la tabella è vuota
	var count int
	db.QueryRow("SELECT COUNT(*) FROM activity_types").Scan(&count)
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
			db.Exec("INSERT INTO activity_types (name, color_variant, pattern, display_order) VALUES (?, ?, ?, ?)",
				t.name, t.colorVariant, t.pattern, t.order)
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
	db.Exec("INSERT OR IGNORE INTO settings (key, value) VALUES ('auto_start', 'false')")

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

	fmt.Println("[DB] Database inizializzato con successo")
	return db, nil

}

// SalvaApp salva un'applicazione nel database
func SalvaApp(db *sql.DB, app Applicazione) error {
	insertSQL := `INSERT INTO applicazioni (nome, tempo, categoria) VALUES (?,?,?)`
	_, err := db.Exec(insertSQL, app.Nome, app.Tempo, app.Categoria)
	if err != nil {
		return fmt.Errorf("errore salvataggio app: %v", err)
	}
	fmt.Printf("[DB] Salvata: %s\n", app.Nome)
	return nil
}

// CaricaTutteApp carica tutte le applicazioni dal database
func CaricaTutteApp(db *sql.DB) ([]Applicazione, error) {
	selectSQL := `SELECT nome, tempo, categoria FROM applicazioni`

	rows, err := db.Query(selectSQL)
	if err != nil {
		return nil, fmt.Errorf("errore lettura database: %v", err)
	}
	defer rows.Close()

	var apps []Applicazione
	for rows.Next() {
		var app Applicazione
		if err := rows.Scan(&app.Nome, &app.Tempo, &app.Categoria); err != nil {
			return nil, fmt.Errorf("errore lettura riga: %v", err)
		}
		apps = append(apps, app)
	}

	return apps, nil
}

// AggiornaTempoApp aggiorna il tempo di un'app
func AggiornaTempoApp(db *sql.DB, nome string, nuovoTempo int) error {
	updateSQL := `UPDATE applicazioni SET tempo = ? WHERE nome = ?`

	result, err := db.Exec(updateSQL, nuovoTempo, nome)
	if err != nil {
		return fmt.Errorf("errore aggiornamento: %v", err)
	}

	// Controlla quante righe sono state modificate
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("app '%s' non trovata", nome)
	}

	fmt.Printf("[DB] Aggiornata: %s -> %d minuti\n", nome, nuovoTempo)
	return nil
}

// EliminaApp elimina un'app dal database
func EliminaApp(db *sql.DB, nome string) error {
	deleteSQL := `DELETE FROM applicazioni WHERE nome = ?`

	result, err := db.Exec(deleteSQL, nome)
	if err != nil {
		return fmt.Errorf("errore eliminazione: %v", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("app '%s' non trovata", nome)
	}

	fmt.Printf("[DB] Eliminata: %s\n", nome)
	return nil
}

// GetDefaultActivityType restituisce il primo tipo di attività secondo l'ordine gerarchico
func GetDefaultActivityType(db *sql.DB) *string {
	query := `SELECT name FROM activity_types ORDER BY display_order ASC LIMIT 1`

	var name string
	err := db.QueryRow(query).Scan(&name)
	if err != nil {
		// Se non ci sono tipi di attività, ritorna nil
		return nil
	}

	return &name
}

// SalvaSessione salva una sessione di tracking
// Assegna automaticamente il primo tipo di attività disponibile
func SalvaSessione(db *sql.DB, appName string, seconds int, projectID *int) error {
	// Ottieni il primo tipo di attività (ordine gerarchico)
	defaultActivityType := GetDefaultActivityType(db)
	return SalvaSessioneConTipo(db, appName, seconds, projectID, "computer", defaultActivityType, "")
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

// SalvaStatistiche salva tutte le statistiche del watcher
func SalvaStatistiche(db *sql.DB, stats map[string]int, projectID *int) error {
	fmt.Println("[DB] Salvataggio statistiche...")

	for appName, seconds := range stats {
		if err := SalvaSessione(db, appName, seconds, projectID); err != nil {
			return err
		}
		fmt.Printf("[DB] Salvata sessione: %s -> %d secondi\n", appName, seconds)
	}

	return nil
}

// CaricaSessioniOggi carica le sessioni di oggi
func CaricaSessioniOggi(db *sql.DB) (map[string]int, error) {
	query := `
	SELECT app_name, SUM(seconds) as total
	FROM sessions
	WHERE DATE(timestamp) = DATE('now')
	GROUP BY app_name
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("errore query sessioni: %v", err)
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var appName string
		var total int
		if err := rows.Scan(&appName, &total); err != nil {
			return nil, err
		}
		stats[appName] = total
	}

	return stats, nil
}

// CaricaSessioniSettimana carica le sessioni della settimana corrente
func CaricaSessioniSettimana(db *sql.DB) (map[string]int, error) {
	query := `
	SELECT app_name, SUM(seconds) as total
	FROM sessions
	WHERE DATE(timestamp) >= DATE('now', 'weekday 0', '-7 days')
	GROUP BY app_name
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("errore query sessioni settimana: %v", err)
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var appName string
		var total int
		if err := rows.Scan(&appName, &total); err != nil {
			return nil, err
		}
		stats[appName] = total
	}

	return stats, nil
}

// CaricaSessioniMese carica le sessioni del mese corrente
func CaricaSessioniMese(db *sql.DB) (map[string]int, error) {
	query := `
	SELECT app_name, SUM(seconds) as total
	FROM sessions
	WHERE strftime('%Y-%m', timestamp) = strftime('%Y-%m', 'now')
	GROUP BY app_name
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("errore query sessioni mese: %v", err)
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var appName string
		var total int
		if err := rows.Scan(&appName, &total); err != nil {
			return nil, err
		}
		stats[appName] = total
	}

	return stats, nil
}

// CaricaTopApp carica le top N app più usate di oggi
func CaricaTopApp(db *sql.DB, limit int) ([]struct {
	Nome    string
	Secondi int
}, error) {
	query := `
	SELECT app_name, SUM(seconds) as total
	FROM sessions
	WHERE DATE(timestamp) = DATE('now')
	GROUP BY app_name
	ORDER BY total DESC
	LIMIT ?
	`

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("errore query top app: %v", err)
	}
	defer rows.Close()

	type AppStat struct {
		Nome    string
		Secondi int
	}

	var topApps []struct {
		Nome    string
		Secondi int
	}

	for rows.Next() {
		var app struct {
			Nome    string
			Secondi int
		}
		if err := rows.Scan(&app.Nome, &app.Secondi); err != nil {
			return nil, err
		}
		topApps = append(topApps, app)
	}

	return topApps, nil
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

// CaricaTuttiProgetti carica tutti i progetti
func CaricaTuttiProgetti(db *sql.DB) ([]Project, error) {
	selectSQL := `SELECT id, name, description, created_at, archived, COALESCE(closed_at, '') FROM projects ORDER BY created_at DESC`

	rows, err := db.Query(selectSQL)
	if err != nil {
		return nil, fmt.Errorf("errore lettura progetti: %v", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		var archived int
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &archived, &p.ClosedAt); err != nil {
			return nil, fmt.Errorf("errore lettura riga: %v", err)
		}
		p.Archived = archived == 1
		projects = append(projects, p)
	}

	return projects, nil
}

// TrovaProgetto cerca un progetto per nome
func TrovaProgetto(db *sql.DB, name string) (*Project, error) {
	selectSQL := `SELECT id, name, description, created_at, archived, COALESCE(closed_at, '') FROM projects WHERE name = ?`

	var p Project
	var archived int
	err := db.QueryRow(selectSQL, name).Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &archived, &p.ClosedAt)
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
	selectSQL := `SELECT id, name, description, created_at, archived, COALESCE(closed_at, '') FROM projects WHERE id = ?`

	var p Project
	var archived int
	err := db.QueryRow(selectSQL, id).Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &archived, &p.ClosedAt)
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
	// Recupera il nome del progetto per il log
	var projectName string
	err := db.QueryRow("SELECT name FROM projects WHERE id = ?", projectID).Scan(&projectName)
	if err != nil {
		return fmt.Errorf("progetto non trovato: %v", err)
	}

	// Elimina tutte le note associate al progetto
	deleteNotesSQL := `DELETE FROM notes WHERE project_id = ?`
	result, err := db.Exec(deleteNotesSQL, projectID)
	if err != nil {
		return fmt.Errorf("errore eliminazione note del progetto: %v", err)
	}
	notesDeleted, _ := result.RowsAffected()

	// Elimina tutte le sessioni associate al progetto
	deleteSessionsSQL := `DELETE FROM sessions WHERE project_id = ?`
	result, err = db.Exec(deleteSessionsSQL, projectID)
	if err != nil {
		return fmt.Errorf("errore eliminazione sessioni del progetto: %v", err)
	}
	sessionsDeleted, _ := result.RowsAffected()

	// Infine, elimina il progetto stesso
	deleteProjectSQL := `DELETE FROM projects WHERE id = ?`
	result, err = db.Exec(deleteProjectSQL, projectID)
	if err != nil {
		return fmt.Errorf("errore eliminazione progetto: %v", err)
	}

	fmt.Printf("[DB] Progetto eliminato: %s (ID: %d, sessioni: %d, note: %d)\n", projectName, projectID, sessionsDeleted, notesDeleted)
	return nil
}

// CaricaSessioniProgetto carica tutte le sessioni per un progetto specifico
func CaricaSessioniProgetto(db *sql.DB, projectID int) (map[string]int, error) {
	query := `
	SELECT app_name, SUM(seconds) as total
	FROM sessions
	WHERE project_id = ?
	GROUP BY app_name
	`

	rows, err := db.Query(query, projectID)
	if err != nil {
		return nil, fmt.Errorf("errore query sessioni progetto: %v", err)
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var appName string
		var total int
		if err := rows.Scan(&appName, &total); err != nil {
			return nil, err
		}
		stats[appName] = total
	}

	return stats, nil
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
func CaricaSessioniDettagliate(db *sql.DB, startDate, endDate string) ([]SessionDetail, error) {
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
	WHERE DATE(s.timestamp) >= DATE(?) AND DATE(s.timestamp) <= DATE(?)
	ORDER BY s.timestamp ASC
	`

	rows, err := db.Query(query, startDate, endDate)
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

// === FUNZIONI PER NOTE ===

// Note rappresenta una nota di progetto
type Note struct {
	ID        int
	ProjectID int
	NoteText  string
	Timestamp string
}

// CreaNote crea una nuova nota per un progetto
func CreaNote(db *sql.DB, projectID int, noteText, timestamp string) (int64, error) {
	var insertSQL string
	var result sql.Result
	var err error

	if timestamp != "" {
		insertSQL = `INSERT INTO notes (project_id, note_text, timestamp) VALUES (?, ?, ?)`
		result, err = db.Exec(insertSQL, projectID, noteText, timestamp)
	} else {
		insertSQL = `INSERT INTO notes (project_id, note_text) VALUES (?, ?)`
		result, err = db.Exec(insertSQL, projectID, noteText)
	}

	if err != nil {
		return 0, fmt.Errorf("errore creazione nota: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("errore ottenimento ID: %v", err)
	}

	fmt.Printf("[DB] Nota creata per progetto ID %d\n", projectID)
	return id, nil
}

// CaricaNoteProgetto carica tutte le note di un progetto
func CaricaNoteProgetto(db *sql.DB, projectID int) ([]Note, error) {
	query := `SELECT id, project_id, note_text, timestamp FROM notes WHERE project_id = ? ORDER BY timestamp DESC`

	rows, err := db.Query(query, projectID)
	if err != nil {
		return nil, fmt.Errorf("errore query note: %v", err)
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.ProjectID, &n.NoteText, &n.Timestamp); err != nil {
			return nil, err
		}
		notes = append(notes, n)
	}

	return notes, nil
}

// CaricaNotePerPeriodo carica le note di tutti i progetti in un periodo
func CaricaNotePerPeriodo(db *sql.DB, startDate, endDate string) ([]Note, error) {
	query := `
	SELECT id, project_id, note_text, timestamp
	FROM notes
	WHERE DATE(timestamp) >= DATE(?) AND DATE(timestamp) <= DATE(?)
	ORDER BY timestamp ASC
	`

	rows, err := db.Query(query, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("errore query note periodo: %v", err)
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.ProjectID, &n.NoteText, &n.Timestamp); err != nil {
			return nil, err
		}
		notes = append(notes, n)
	}

	return notes, nil
}

// CaricaTutteLeNote carica tutte le note con filtri opzionali
func CaricaTutteLeNote(db *sql.DB, projectID, searchText, limit string) ([]Note, error) {
	query := `
	SELECT id, project_id, note_text, timestamp
	FROM notes
	WHERE 1=1
	`

	args := []interface{}{}

	// Filtro per progetto
	if projectID != "" {
		query += " AND project_id = ?"
		args = append(args, projectID)
	}

	// Filtro per ricerca testuale
	if searchText != "" {
		query += " AND note_text LIKE ?"
		args = append(args, "%"+searchText+"%")
	}

	query += " ORDER BY timestamp DESC"

	// Limite risultati
	if limit != "" {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("errore query tutte le note: %v", err)
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.ProjectID, &n.NoteText, &n.Timestamp); err != nil {
			return nil, err
		}
		notes = append(notes, n)
	}

	return notes, nil
}

// AggiornaNota aggiorna il testo di una nota esistente
func AggiornaNota(db *sql.DB, noteID int, noteText string) error {
	updateSQL := `UPDATE notes SET note_text = ? WHERE id = ?`
	_, err := db.Exec(updateSQL, noteText, noteID)
	if err != nil {
		return fmt.Errorf("errore aggiornamento nota: %v", err)
	}
	fmt.Printf("[DB] Nota ID %d aggiornata\n", noteID)
	return nil
}

// EliminaNota elimina una nota dal database
func EliminaNota(db *sql.DB, noteID int) error {
	deleteSQL := `DELETE FROM notes WHERE id = ?`
	_, err := db.Exec(deleteSQL, noteID)
	if err != nil {
		return fmt.Errorf("errore eliminazione nota: %v", err)
	}
	fmt.Printf("[DB] Nota ID %d eliminata\n", noteID)
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
	projectSQL := `SELECT id, name, description, created_at, archived, COALESCE(closed_at, '') FROM projects WHERE id = ?`
	err := db.QueryRow(projectSQL, projectID).Scan(&project.ID, &project.Name, &project.Description, &project.CreatedAt, &archived, &project.ClosedAt)
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

// DividiSessione divide una sessione in due parti
func DividiSessione(db *sql.DB, sessionID int, secondiPrimaParte int, activityTypePrimaParte *string, activityTypeSecondaParte *string) error {
	// Prima carica la sessione originale
	var appName, sessionType, timestamp string
	var seconds int
	var projectID *int
	var activityType *string

	query := `SELECT app_name, seconds, project_id, session_type, activity_type, timestamp FROM sessions WHERE id = ?`
	err := db.QueryRow(query, sessionID).Scan(&appName, &seconds, &projectID, &sessionType, &activityType, &timestamp)
	if err != nil {
		return fmt.Errorf("errore caricamento sessione: %v", err)
	}

	if secondiPrimaParte >= seconds {
		return fmt.Errorf("la prima parte deve essere minore della durata totale")
	}

	secondiSecondaParte := seconds - secondiPrimaParte

	// Aggiorna la sessione originale con la prima parte
	updateSQL := `UPDATE sessions SET seconds = ?, activity_type = ? WHERE id = ?`
	_, err = db.Exec(updateSQL, secondiPrimaParte, activityTypePrimaParte, sessionID)
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
	_, err = db.Exec(insertSQL, appName, secondiSecondaParte, projectID, sessionType, activityTypeSecondaParte, timestampSecondaParteStr)
	if err != nil {
		return fmt.Errorf("errore creazione seconda parte: %v", err)
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
	// Crea la sessione con 0 secondi (verrà aggiornata periodicamente)
	insertSessionSQL := `INSERT INTO sessions (app_name, seconds, project_id, session_type, activity_type, timestamp) VALUES (?, ?, ?, ?, ?, ?)`
	result, err := db.Exec(insertSessionSQL, "Sessione di lavoro", 0, projectID, "computer", activityType, startTime)
	if err != nil {
		return 0, fmt.Errorf("errore creazione sessione pendente: %v", err)
	}

	sessionID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("errore recupero ID sessione: %v", err)
	}

	// Registra il pending tracking
	insertPendingSQL := `INSERT INTO pending_tracking (session_id, project_id, activity_type, start_time, last_saved_seconds) VALUES (?, ?, ?, ?, ?)`
	_, err = db.Exec(insertPendingSQL, sessionID, projectID, activityType, startTime, 0)
	if err != nil {
		// Rollback: elimina la sessione creata
		db.Exec("DELETE FROM sessions WHERE id = ?", sessionID)
		return 0, fmt.Errorf("errore registrazione pending tracking: %v", err)
	}

	fmt.Printf("[DB] Pending tracking avviato - Session ID: %d\n", sessionID)
	return sessionID, nil
}

// UpdatePendingTracking aggiorna i secondi della sessione pendente
func UpdatePendingTracking(db *sql.DB, sessionID int64, totalSeconds int) error {
	// Aggiorna i secondi nella sessione
	updateSessionSQL := `UPDATE sessions SET seconds = ? WHERE id = ?`
	_, err := db.Exec(updateSessionSQL, totalSeconds, sessionID)
	if err != nil {
		return fmt.Errorf("errore aggiornamento sessione: %v", err)
	}

	// Aggiorna last_saved_seconds e last_update nel pending tracking
	updatePendingSQL := `UPDATE pending_tracking SET last_saved_seconds = ?, last_update = CURRENT_TIMESTAMP WHERE session_id = ?`
	_, err = db.Exec(updatePendingSQL, totalSeconds, sessionID)
	if err != nil {
		return fmt.Errorf("errore aggiornamento pending tracking: %v", err)
	}

	fmt.Printf("[DB] Pending tracking aggiornato - Session ID: %d, Secondi: %d\n", sessionID, totalSeconds)
	return nil
}

// FinalizePendingTracking finalizza la sessione e rimuove il pending tracking
func FinalizePendingTracking(db *sql.DB, sessionID int64, finalSeconds int) error {
	// Aggiorna i secondi finali nella sessione
	updateSessionSQL := `UPDATE sessions SET seconds = ? WHERE id = ?`
	_, err := db.Exec(updateSessionSQL, finalSeconds, sessionID)
	if err != nil {
		return fmt.Errorf("errore finalizzazione sessione: %v", err)
	}

	// Rimuovi il pending tracking
	deletePendingSQL := `DELETE FROM pending_tracking WHERE session_id = ?`
	_, err = db.Exec(deletePendingSQL, sessionID)
	if err != nil {
		return fmt.Errorf("errore rimozione pending tracking: %v", err)
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
			// Aggiorna la sessione con i secondi salvati
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
