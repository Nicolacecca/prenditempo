package tracker

import (
	"database/sql"
	"fmt"
	"time"
)

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
	unisciDopoScrittura(db, int64(sessionID))
	return nil
}

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
	unisciDopoScrittura(db, int64(sessionID))
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
	unisciDopoScrittura(db, int64(sessionID))
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
	timestampOriginale, err := parseNaiveTimestamp(timestamp)
	if err != nil {
		return fmt.Errorf("errore parsing timestamp: %v", err)
	}

	timestampSecondaParte := timestampOriginale.Add(time.Duration(secondiPrimaParte) * time.Second)
	timestampSecondaParteStr := timestampSecondaParte.Format(TimestampLayout)

	// Crea una nuova sessione per la seconda parte
	insertSQL := `INSERT INTO sessions (app_name, seconds, project_id, session_type, activity_type, timestamp) VALUES (?, ?, ?, ?, ?, ?)`
	insertResult, err := tx.Exec(insertSQL, appName, secondiSecondaParte, projectID, sessionType, activityTypeSecondaParte, timestampSecondaParteStr)
	if err != nil {
		return fmt.Errorf("errore creazione seconda parte: %v", err)
	}
	secondPartID, _ := insertResult.LastInsertId()

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("errore conferma divisione sessione: %v", err)
	}
	fmt.Printf("[DB] Sessione ID %d divisa in due parti: %d sec e %d sec\n", sessionID, secondiPrimaParte, secondiSecondaParte)
	// Le due parti si riuniscono solo se hanno lo stesso tipo; si fondono anche con le vicine.
	unisciDopoScrittura(db, int64(sessionID), secondPartID)
	return nil
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
	unisciDopoScrittura(db, sessionID)
	return nil
}
