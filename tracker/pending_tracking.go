package tracker

import (
	"database/sql"
	"fmt"
)

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
	unisciDopoScrittura(db, sessionID)
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
		if err := recoverOnePending(db, p); err != nil {
			fmt.Printf("[DB] Errore recupero sessione pendente ID %d: %v\n", p.SessionID, err)
			continue
		}
		recovered++
	}

	if recovered > 0 {
		fmt.Printf("[DB] Recuperate %d sessioni pendenti\n", recovered)
	}

	return recovered, nil
}

// recoverOnePending finalizza o scarta una sessione pendente in un'unica transazione:
// una chiusura a metà non può lasciare sessione e pending incoerenti. Ripetibile.
func recoverOnePending(db *sql.DB, p PendingTracking) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if p.LastSavedSeconds > 0 {
		// Si recupera solo fino all'ultimo checkpoint: non si estende all'ora di
		// riavvio perché il computer potrebbe essere rimasto spento. L'UPDATE precede
		// la rimozione del pending, che esenta la sessione dal controllo sovrapposizioni.
		if _, err := tx.Exec(`UPDATE sessions SET seconds = ? WHERE id = ?`, p.LastSavedSeconds, p.SessionID); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM pending_tracking WHERE id = ?`, p.ID); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		unisciDopoScrittura(db, int64(p.SessionID))
		return nil
	}
	// Sessione vuota: il pending ha una foreign key verso la sessione, va tolto prima.
	if _, err := tx.Exec(`DELETE FROM pending_tracking WHERE id = ?`, p.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM sessions WHERE id = ?`, p.SessionID); err != nil {
		return err
	}
	return tx.Commit()
}
