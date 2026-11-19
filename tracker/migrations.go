package tracker

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// migration è un passaggio di schema applicato una sola volta, in transazione.
// Lo stato è tracciato da PRAGMA user_version: per cambiare lo schema si AGGIUNGE
// una migrazione in fondo a `migrations`, non si modificano quelle già rilasciate.
type migration struct {
	version int
	name    string
	apply   func(tx *sql.Tx) error
}

var migrations = []migration{
	{1, "schema base e colonne storiche", migrateBaseline},
	{2, "vincoli di validazione (trigger)", migrateTriggers},
}

// runMigrations porta il database all'ultima versione. Ogni migrazione e il relativo
// aggiornamento di user_version stanno nella stessa transazione: se fallisce, il
// database resta alla versione precedente. Prima di migrare un database esistente
// viene creata una copia in backups/.
func runMigrations(db *sql.DB, dbPath string, migs []migration) error {
	var current int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&current); err != nil {
		return fmt.Errorf("errore lettura versione schema: %v", err)
	}
	latest := migs[len(migs)-1].version
	if current > latest {
		return fmt.Errorf("il database è stato creato da una versione più recente di PrendiTempo (schema %d, supportato %d): aggiorna l'applicazione", current, latest)
	}
	if current == latest {
		return nil
	}
	if err := backupBeforeMigration(db, dbPath, current); err != nil {
		return err
	}

	for _, m := range migs {
		if m.version <= current {
			continue
		}
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("errore avvio migrazione %d (%s): %v", m.version, m.name, err)
		}
		if err := m.apply(tx); err != nil {
			tx.Rollback()
			return fmt.Errorf("migrazione %d (%s) fallita: %v", m.version, m.name, err)
		}
		if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, m.version)); err != nil {
			tx.Rollback()
			return fmt.Errorf("errore aggiornamento versione schema %d: %v", m.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("errore conferma migrazione %d (%s): %v", m.version, m.name, err)
		}
		fmt.Printf("[DB] Migrazione %d applicata: %s\n", m.version, m.name)
	}
	return nil
}

// backupBeforeMigration copia un database esistente (non uno nuovo) in backups/
// prima di modificarne lo schema. VACUUM INTO produce uno snapshot coerente anche in WAL.
func backupBeforeMigration(db *sql.DB, dbPath string, fromVersion int) error {
	if dbPath == "" || dbPath == ":memory:" {
		return nil
	}
	var tables int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'projects'`).Scan(&tables); err != nil {
		return fmt.Errorf("errore verifica database esistente: %v", err)
	}
	if tables == 0 {
		return nil // database nuovo: niente da proteggere
	}
	dir := filepath.Join(filepath.Dir(dbPath), "backups")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("errore creazione cartella backup migrazione: %v", err)
	}
	target := filepath.Join(dir, fmt.Sprintf("pre-migrazione-v%d-%s.db", fromVersion, time.Now().Format("20060102-150405")))
	if _, err := db.Exec(`VACUUM INTO ?`, target); err != nil {
		return fmt.Errorf("backup prima della migrazione non riuscito: %v", err)
	}
	fmt.Printf("[DB] Backup pre-migrazione creato: %s\n", target)
	return nil
}

// migrateBaseline porta qualsiasi database storico (non versionato) allo schema
// attuale. È volutamente idempotente: i database esistenti hanno già parte di
// tabelle e colonne, quindi "duplicate column" è tollerato.
func migrateBaseline(tx *sql.Tx) error {
	// Crea la tabella se non esiste
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS applicazioni (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		nome TEXT NOT NULL,
		tempo INTEGER NOT NULL,
		categoria TEXT NOT NULL,
		data_creazione DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err := tx.Exec(createTableSQL); err != nil {
		return fmt.Errorf("errore creazione tabella: %v", err)
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

	if _, err := tx.Exec(createProjectsSQL); err != nil {
		return fmt.Errorf("errore creazione tabella projects: %v", err)
	}

	// Migrazione: aggiungi colonne archived e closed_at se non esistono
	// Ignoriamo errori "duplicate column" che indicano che la colonna esiste già
	if _, err := tx.Exec(`ALTER TABLE projects ADD COLUMN archived INTEGER DEFAULT 0;`); err != nil {
		if !isDuplicateColumnError(err) {
			return fmt.Errorf("errore migrazione projects.archived: %v", err)
		}
	}
	if _, err := tx.Exec(`ALTER TABLE projects ADD COLUMN closed_at DATETIME;`); err != nil {
		if !isDuplicateColumnError(err) {
			return fmt.Errorf("errore migrazione projects.closed_at: %v", err)
		}
	}
	// Migrazione: aggiungi colonna note_text per nota markdown del progetto
	if _, err := tx.Exec(`ALTER TABLE projects ADD COLUMN note_text TEXT DEFAULT '';`); err != nil {
		if !isDuplicateColumnError(err) {
			return fmt.Errorf("errore migrazione projects.note_text: %v", err)
		}
	}
	// Migrazione: aggiungi colonna board_url, il link alla lavagna esterna
	// (Excalidraw o altro provider a scelta dell'utente) collegata al progetto
	if _, err := tx.Exec(`ALTER TABLE projects ADD COLUMN board_url TEXT DEFAULT '';`); err != nil {
		if !isDuplicateColumnError(err) {
			return fmt.Errorf("errore migrazione projects.board_url: %v", err)
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

	if _, err := tx.Exec(createSessionsSQL); err != nil {
		return fmt.Errorf("errore creazione tabella sessions: %v", err)
	}

	// Migrazione: aggiungi colonna activity_type se non esiste
	if _, err := tx.Exec(`ALTER TABLE sessions ADD COLUMN activity_type TEXT DEFAULT NULL;`); err != nil {
		if !isDuplicateColumnError(err) {
			return fmt.Errorf("errore migrazione sessions.activity_type: %v", err)
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

	if _, err := tx.Exec(createNotesSQL); err != nil {
		return fmt.Errorf("errore creazione tabella notes: %v", err)
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

	if _, err := tx.Exec(createProjectBoardsSQL); err != nil {
		return fmt.Errorf("errore creazione tabella project_boards: %v", err)
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

	if _, err := tx.Exec(createBoardAssetsSQL); err != nil {
		return fmt.Errorf("errore creazione tabella board_assets: %v", err)
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

	if _, err := tx.Exec(createBoardVersionsSQL); err != nil {
		return fmt.Errorf("errore creazione tabella board_versions: %v", err)
	}
	if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_board_versions_project_date ON board_versions(project_id, created_at DESC, id DESC)`); err != nil {
		return fmt.Errorf("errore creazione indice board_versions: %v", err)
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

	if _, err := tx.Exec(createActivityTypesSQL); err != nil {
		return fmt.Errorf("errore creazione tabella activity_types: %v", err)
	}

	// Migrazione: aggiungi colonna pattern se non esiste
	if _, err := tx.Exec(`ALTER TABLE activity_types ADD COLUMN pattern TEXT DEFAULT 'solid';`); err != nil {
		if !isDuplicateColumnError(err) {
			return fmt.Errorf("errore migrazione activity_types.pattern: %v", err)
		}
	}

	// Migrazione: archived permette di ritirare un tipo già usato senza perdere lo storico
	if _, err := tx.Exec(`ALTER TABLE activity_types ADD COLUMN archived INTEGER DEFAULT 0;`); err != nil {
		if !isDuplicateColumnError(err) {
			return fmt.Errorf("errore migrazione activity_types.archived: %v", err)
		}
	}

	// Inserisci tipi di attività di default se la tabella è vuota
	var count int
	if err := tx.QueryRow("SELECT COUNT(*) FROM activity_types").Scan(&count); err != nil {
		return fmt.Errorf("errore conteggio tipi attività: %v", err)
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
			if _, err := tx.Exec("INSERT INTO activity_types (name, color_variant, pattern, display_order) VALUES (?, ?, ?, ?)",
				t.name, t.colorVariant, t.pattern, t.order); err != nil {
				return fmt.Errorf("errore inserimento tipo attività predefinito: %v", err)
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

	if _, err := tx.Exec(createSettingsSQL); err != nil {
		return fmt.Errorf("errore creazione tabella settings: %v", err)
	}

	// Inserisci impostazioni di default se non esistono
	if _, err := tx.Exec("INSERT OR IGNORE INTO settings (key, value) VALUES ('auto_start', 'false')"); err != nil {
		return fmt.Errorf("errore impostazioni predefinite: %v", err)
	}

	// Crea indici per migliorare le performance delle query
	for _, statement := range []string{
		`CREATE INDEX IF NOT EXISTS idx_sessions_timestamp ON sessions(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_project_id ON sessions(project_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_app_name ON sessions(app_name)`,
		`CREATE INDEX IF NOT EXISTS idx_notes_project_id ON notes(project_id)`,
		`CREATE INDEX IF NOT EXISTS idx_notes_timestamp ON notes(timestamp)`,
	} {
		if _, err := tx.Exec(statement); err != nil {
			return fmt.Errorf("errore creazione indice: %v", err)
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
	if _, err := tx.Exec(createDayEventsSQL); err != nil {
		return fmt.Errorf("errore creazione tabella day_events: %v", err)
	}
	if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_day_events_date ON day_events(date)`); err != nil {
		return fmt.Errorf("errore indice eventi: %v", err)
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

	if _, err := tx.Exec(createPendingTrackingSQL); err != nil {
		return fmt.Errorf("errore creazione tabella pending_tracking: %v", err)
	}
	return nil
}

// migrateTriggers (ri)crea i trigger di validazione. Per cambiarne uno già rilasciato
// serve una NUOVA migrazione che lo droppa e lo ricrea: "CREATE TRIGGER IF NOT EXISTS"
// da solo non aggiornerebbe mai un trigger già presente.
func migrateTriggers(tx *sql.Tx) error {
	// I trigger su day_events vengono ricreati ad ogni avvio (drop + create) per
	// permettere di aggiornare l'elenco di event_type ammessi (es. aggiunta di
	// 'malattia') anche sui database esistenti: "CREATE TRIGGER IF NOT EXISTS"
	// da solo non aggiornerebbe mai un trigger già presente.
	for _, statement := range []string{
		`DROP TRIGGER IF EXISTS validate_day_events_insert`,
		`DROP TRIGGER IF EXISTS validate_day_events_update`,
		`DROP TRIGGER IF EXISTS validate_sessions_no_overlap_insert`,
		`DROP TRIGGER IF EXISTS validate_sessions_no_overlap_update`,
	} {
		if _, err := tx.Exec(statement); err != nil {
			return fmt.Errorf("errore aggiornamento vincoli database: %v", err)
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
		// Impedisce sessioni che si sovrappongono nel tempo reale (intervalli [inizio, fine)):
		// senza questo vincolo due sessioni concorrenti gonfiano i totali di ore.
		// Copre creazione manuale, modifica, split e import (tutti scrivono su sessions).
		// Il tracking in corso e' esente dal controllo mentre cresce (checkpoint e
		// finalizzazione non devono mai fallire e perdere dati): al suo posto si rifiuta
		// ogni altra sessione che termini dopo l'inizio del tracking attivo.
		`CREATE TRIGGER validate_sessions_no_overlap_insert BEFORE INSERT ON sessions
		 WHEN EXISTS (
		     SELECT 1 FROM sessions
		     WHERE timestamp < datetime(NEW.timestamp, '+' || NEW.seconds || ' seconds')
		       AND datetime(timestamp, '+' || seconds || ' seconds') > NEW.timestamp
		 ) OR EXISTS (
		     SELECT 1 FROM pending_tracking p JOIN sessions s ON s.id = p.session_id
		     WHERE datetime(NEW.timestamp, '+' || NEW.seconds || ' seconds') > s.timestamp
		 )
		 BEGIN SELECT RAISE(ABORT, 'sessione sovrapposta a una sessione esistente o al tracking in corso'); END`,
		`CREATE TRIGGER validate_sessions_no_overlap_update BEFORE UPDATE ON sessions
		 WHEN (NEW.timestamp != OLD.timestamp OR NEW.seconds != OLD.seconds)
		   AND NEW.id NOT IN (SELECT session_id FROM pending_tracking)
		   AND (EXISTS (
		     SELECT 1 FROM sessions
		     WHERE id != NEW.id
		       AND timestamp < datetime(NEW.timestamp, '+' || NEW.seconds || ' seconds')
		       AND datetime(timestamp, '+' || seconds || ' seconds') > NEW.timestamp
		   ) OR EXISTS (
		     SELECT 1 FROM pending_tracking p JOIN sessions s ON s.id = p.session_id
		     WHERE datetime(NEW.timestamp, '+' || NEW.seconds || ' seconds') > s.timestamp
		   ))
		 BEGIN SELECT RAISE(ABORT, 'sessione sovrapposta a una sessione esistente o al tracking in corso'); END`,
	} {
		if _, err := tx.Exec(statement); err != nil {
			return fmt.Errorf("errore creazione vincoli database: %v", err)
		}
	}
	return nil
}
