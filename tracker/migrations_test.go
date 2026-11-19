package tracker

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func schemaVersionOf(t *testing.T, db *sql.DB) int {
	t.Helper()
	var v int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func latestSchemaVersion() int { return migrations[len(migrations)-1].version }

func hasColumn(t *testing.T, db *sql.DB, table, column string) bool {
	t.Helper()
	rows, err := db.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		rows.Scan(&name)
		if name == column {
			return true
		}
	}
	return false
}

func preMigrationBackups(t *testing.T, dbPath string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(filepath.Dir(dbPath), "backups", "pre-migrazione-*.db"))
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestMigrationsEmptyDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.db")
	db, err := InitDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if v := schemaVersionOf(t, db); v != latestSchemaVersion() {
		t.Fatalf("versione schema = %d, attesa %d", v, latestSchemaVersion())
	}
	if len(preMigrationBackups(t, path)) != 0 {
		t.Fatal("un database nuovo non richiede backup")
	}
}

func TestMigrationsReopenIsNoOp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reopen.db")
	db, err := InitDB(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreaProgetto(db, "Dati", ""); err != nil {
		t.Fatal(err)
	}
	db.Close()

	db, err = InitDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	db.QueryRow(`SELECT COUNT(*) FROM projects`).Scan(&n)
	if n != 1 || schemaVersionOf(t, db) != latestSchemaVersion() {
		t.Fatalf("riapertura: progetti=%d versione=%d", n, schemaVersionOf(t, db))
	}
	if len(preMigrationBackups(t, path)) != 0 {
		t.Fatal("la riapertura di un database già migrato non deve creare backup")
	}
}

func TestMigrationsUpgradeCurrentUnversionedDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "current.db")
	db, err := InitDB(path)
	if err != nil {
		t.Fatal(err)
	}
	CreaProgetto(db, "Esistente", "")
	// Simula un'installazione attuale, creata prima del versionamento.
	if _, err := db.Exec(`PRAGMA user_version = 0`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	db, err = InitDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	db.QueryRow(`SELECT COUNT(*) FROM projects`).Scan(&n)
	if n != 1 || schemaVersionOf(t, db) != latestSchemaVersion() {
		t.Fatalf("progetti=%d versione=%d", n, schemaVersionOf(t, db))
	}
	if len(preMigrationBackups(t, path)) != 1 {
		t.Fatal("atteso un backup prima di migrare un database esistente")
	}
}

func TestMigrationsUpgradeOldSchemaFixture(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE projects (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL UNIQUE, description TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE sessions (id INTEGER PRIMARY KEY AUTOINCREMENT, app_name TEXT NOT NULL, seconds INTEGER NOT NULL, project_id INTEGER, session_type TEXT DEFAULT 'computer', timestamp DATETIME DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY (project_id) REFERENCES projects(id))`,
		`CREATE TABLE activity_types (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL UNIQUE, color_variant REAL DEFAULT 0.0, display_order INTEGER DEFAULT 0, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`INSERT INTO projects (name, description) VALUES ('Vecchio', 'dati storici')`,
		`INSERT INTO sessions (app_name, seconds, project_id, session_type, timestamp) VALUES ('S', 600, 1, 'manual', '2025-01-10 09:00:00')`,
		`INSERT INTO activity_types (name, display_order) VALUES ('CUSTOM', 1)`,
	} {
		if _, err := raw.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	raw.Close()

	db, err := InitDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for table, column := range map[string]string{
		"projects": "board_url", "sessions": "activity_type", "activity_types": "archived",
	} {
		if !hasColumn(t, db, table, column) {
			t.Fatalf("colonna %s.%s mancante dopo la migrazione", table, column)
		}
	}
	var desc string
	var seconds int
	db.QueryRow(`SELECT description FROM projects WHERE name = 'Vecchio'`).Scan(&desc)
	db.QueryRow(`SELECT seconds FROM sessions WHERE app_name = 'S'`).Scan(&seconds)
	if desc != "dati storici" || seconds != 600 {
		t.Fatalf("dati storici alterati: desc=%q seconds=%d", desc, seconds)
	}
	if schemaVersionOf(t, db) != latestSchemaVersion() {
		t.Fatalf("versione schema = %d", schemaVersionOf(t, db))
	}

	backups := preMigrationBackups(t, path)
	if len(backups) != 1 {
		t.Fatalf("attesi 1 backup, trovati %d", len(backups))
	}
	bak, err := sql.Open("sqlite", backups[0])
	if err != nil {
		t.Fatal(err)
	}
	defer bak.Close()
	var bakProjects int
	if err := bak.QueryRow(`SELECT COUNT(*) FROM projects`).Scan(&bakProjects); err != nil || bakProjects != 1 {
		t.Fatalf("il backup deve contenere il database originale: n=%d err=%v", bakProjects, err)
	}
	if hasColumn(t, bak, "projects", "board_url") {
		t.Fatal("il backup deve riflettere lo schema precedente alla migrazione")
	}
}

func TestMigrationsRejectNewerDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "future.db")
	db, err := InitDB(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`PRAGMA user_version = 999`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	if db, err := InitDB(path); err == nil {
		db.Close()
		t.Fatal("atteso rifiuto di un database creato da una versione più recente")
	} else if !strings.Contains(err.Error(), "più recente") {
		t.Fatalf("messaggio inatteso: %v", err)
	}
}

func TestMigrationFailureRollsBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fail.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	migs := []migration{
		{1, "ok", func(tx *sql.Tx) error { _, err := tx.Exec(`CREATE TABLE a (x INTEGER)`); return err }},
		{2, "rotta", func(tx *sql.Tx) error {
			if _, err := tx.Exec(`CREATE TABLE b (x INTEGER)`); err != nil {
				return err
			}
			_, err := tx.Exec(`INSERT INTO tabella_inesistente VALUES (1)`)
			return err
		}},
	}
	if err := runMigrations(db, path, migs); err == nil {
		t.Fatal("atteso errore dalla migrazione rotta")
	}
	if v := schemaVersionOf(t, db); v != 1 {
		t.Fatalf("la versione deve restare 1 dopo il fallimento, trovata %d", v)
	}
	var n int
	db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name = 'b'`).Scan(&n)
	if n != 0 {
		t.Fatal("la tabella della migrazione fallita deve essere annullata (rollback)")
	}
}
