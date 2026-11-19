package tracker

import (
	"database/sql"
	"fmt"
	"strings"

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
func InitDB(dbPath string) (*sql.DB, error) {
	// Apri connessione al database
	db, err := sql.Open("sqlite", dbPath)
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

	if err := runMigrations(db, dbPath, migrations); err != nil {
		db.Close()
		return nil, err
	}

	fmt.Println("[DB] Database inizializzato con successo")
	return db, nil

}
