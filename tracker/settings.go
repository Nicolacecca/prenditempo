package tracker

import (
	"database/sql"
	"fmt"
)

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
