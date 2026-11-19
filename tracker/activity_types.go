package tracker

import (
	"database/sql"
	"fmt"
)

// ActivityType rappresenta un tipo di attività configurabile
type ActivityType struct {
	ID           int
	Name         string
	ColorVariant float64
	Pattern      string
	DisplayOrder int
	CreatedAt    string
	Archived     bool
}

// CaricaTipiAttivita carica i tipi di attività attivi (proponibili per nuove sessioni)
func CaricaTipiAttivita(db *sql.DB) ([]ActivityType, error) {
	return caricaTipiAttivita(db, false)
}

// CaricaTuttiTipiAttivita include anche i tipi archiviati, necessari per colorare lo storico
func CaricaTuttiTipiAttivita(db *sql.DB) ([]ActivityType, error) {
	return caricaTipiAttivita(db, true)
}

func caricaTipiAttivita(db *sql.DB, includeArchived bool) ([]ActivityType, error) {
	query := `SELECT id, name, color_variant, COALESCE(pattern, 'solid'), display_order, COALESCE(created_at, ''), COALESCE(archived, 0) FROM activity_types`
	if !includeArchived {
		query += ` WHERE COALESCE(archived, 0) = 0`
	}
	query += ` ORDER BY display_order ASC`

	rows, err := db.Query(query)
	if err != nil {
		fmt.Printf("[DB] Errore query tipi attività: %v\n", err)
		return nil, fmt.Errorf("errore query tipi attività: %v", err)
	}
	defer rows.Close()

	var types []ActivityType
	for rows.Next() {
		var t ActivityType
		if err := rows.Scan(&t.ID, &t.Name, &t.ColorVariant, &t.Pattern, &t.DisplayOrder, &t.CreatedAt, &t.Archived); err != nil {
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

	// Un tipo archiviato con lo stesso nome (UNIQUE) viene riattivato: riprende lo storico.
	var archivedID int64
	if err := db.QueryRow(`SELECT id FROM activity_types WHERE name = ? AND COALESCE(archived, 0) = 1`, name).Scan(&archivedID); err == nil {
		if _, err := db.Exec(`UPDATE activity_types SET archived = 0, color_variant = ?, pattern = ?, display_order = ? WHERE id = ?`,
			colorVariant, pattern, displayOrder, archivedID); err != nil {
			return 0, fmt.Errorf("errore riattivazione tipo attività: %v", err)
		}
		return archivedID, nil
	}

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

// AggiornaTipoAttivita aggiorna un tipo di attività esistente.
// Una rinomina propaga il nuovo nome anche alle sessioni già salvate (e al tracking
// in corso): lo storico resta riconducibile allo stesso tipo, senza etichette orfane.
func AggiornaTipoAttivita(db *sql.DB, id int, name string, colorVariant float64, pattern string, displayOrder int) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("errore avvio transazione tipo attività: %v", err)
	}
	defer tx.Rollback()

	var oldName string
	if err := tx.QueryRow(`SELECT name FROM activity_types WHERE id = ?`, id).Scan(&oldName); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("tipo attività non trovato")
		}
		return fmt.Errorf("errore lettura tipo attività: %v", err)
	}

	if _, err := tx.Exec(`UPDATE activity_types SET name = ?, color_variant = ?, pattern = ?, display_order = ? WHERE id = ?`,
		name, colorVariant, pattern, displayOrder, id); err != nil {
		return fmt.Errorf("errore aggiornamento tipo attività: %v", err)
	}
	if oldName != name {
		if _, err := tx.Exec(`UPDATE sessions SET activity_type = ? WHERE activity_type = ?`, name, oldName); err != nil {
			return fmt.Errorf("errore aggiornamento sessioni: %v", err)
		}
		if _, err := tx.Exec(`UPDATE pending_tracking SET activity_type = ? WHERE activity_type = ?`, name, oldName); err != nil {
			return fmt.Errorf("errore aggiornamento tracking in corso: %v", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("errore conferma aggiornamento tipo attività: %v", err)
	}

	fmt.Printf("[DB] Tipo attività ID %d aggiornato\n", id)
	return nil
}

// EliminaTipoAttivita rimuove un tipo di attività mai usato; se è già presente nello
// storico lo archivia (restituisce true) così i report passati non cambiano.
func EliminaTipoAttivita(db *sql.DB, id int) (archived bool, err error) {
	tx, err := db.Begin()
	if err != nil {
		return false, fmt.Errorf("errore avvio transazione tipo attività: %v", err)
	}
	defer tx.Rollback()

	var name string
	if err := tx.QueryRow(`SELECT name FROM activity_types WHERE id = ?`, id).Scan(&name); err != nil {
		if err == sql.ErrNoRows {
			return false, fmt.Errorf("tipo attività non trovato")
		}
		return false, fmt.Errorf("errore lettura tipo attività: %v", err)
	}

	var used int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM sessions WHERE activity_type = ?`, name).Scan(&used); err != nil {
		return false, fmt.Errorf("errore verifica utilizzo tipo attività: %v", err)
	}

	if used > 0 {
		if _, err := tx.Exec(`UPDATE activity_types SET archived = 1 WHERE id = ?`, id); err != nil {
			return false, fmt.Errorf("errore archiviazione tipo attività: %v", err)
		}
		archived = true
	} else if _, err := tx.Exec(`DELETE FROM activity_types WHERE id = ?`, id); err != nil {
		return false, fmt.Errorf("errore eliminazione tipo attività: %v", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("errore conferma eliminazione tipo attività: %v", err)
	}

	fmt.Printf("[DB] Tipo attività ID %d rimosso (archiviato=%v)\n", id, archived)
	return archived, nil
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
