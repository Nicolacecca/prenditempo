package tracker

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"
)

// Intervallo massimo (in secondi) tra due sessioni perché vengano unite. 0 = solo contigue.
const (
	MergeGapSettingKey     = "merge_gap_seconds"
	DefaultMergeGapSeconds = 60
	MaxMergeGapSeconds     = 3600
)

// MergeGapSeconds legge l'intervallo di unione dalle impostazioni (default se assente o non valido).
func MergeGapSeconds(db *sql.DB) int {
	value, err := GetSetting(db, MergeGapSettingKey)
	if err != nil || value == "" {
		return DefaultMergeGapSeconds
	}
	gap, err := strconv.Atoi(value)
	if err != nil || gap < 0 || gap > MaxMergeGapSeconds {
		return DefaultMergeGapSeconds
	}
	return gap
}

// UnisciSessioniAdiacenti fonde la sessione con la precedente e/o la successiva quando
// hanno lo stesso progetto, lo stesso tipo di attività e lo stesso tipo di sessione
// (computer / off-computer / manual) e sono separate da non più di MergeGapSeconds
// (0 = devono toccarsi esattamente).
//
// La sessione unita copre tutto l'arco dall'inizio della prima alla fine dell'ultima,
// quindi l'eventuale intervallo tra le due viene assorbito (al massimo MergeGapSeconds in
// più sul totale). Se nell'intervallo cade un'altra sessione, non si unisce. La sessione
// più antica sopravvive. Il tracking in corso non viene mai fuso. Il vincolo
// anti-sovrapposizione vieta di allungare una sessione sopra un'altra, quindi la vicina
// si elimina prima di allungare quella che resta.
func UnisciSessioniAdiacenti(db *sql.DB, sessionID int64) error {
	gap := MergeGapSeconds(db)

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("errore avvio unione sessioni: %v", err)
	}
	defer tx.Rollback()

	var timestamp, sessionType string
	var seconds int
	var projectID *int
	var activityType *string
	err = tx.QueryRow(`
		SELECT timestamp, seconds, project_id, session_type, activity_type
		FROM sessions
		WHERE id = ? AND id NOT IN (SELECT session_id FROM pending_tracking)`, sessionID).
		Scan(&timestamp, &seconds, &projectID, &sessionType, &activityType)
	if err == sql.ErrNoRows || (err == nil && seconds <= 0) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("errore lettura sessione da unire: %v", err)
	}
	start, err := parseNaiveTimestamp(timestamp)
	if err != nil {
		return err
	}
	span := time.Duration(gap) * time.Second

	// IS confronta anche i NULL (nessun progetto / nessun tipo attività).
	const sameKind = `id != ? AND seconds > 0 AND id NOT IN (SELECT session_id FROM pending_tracking)
		AND project_id IS ? AND activity_type IS ? AND session_type = ?`
	const layout = TimestampLayout

	// gapIsFree: nessun'altra sessione (tracking in corso compreso) cade tra le due.
	gapIsFree := func(from, to time.Time, ignoreA, ignoreB int64) (bool, error) {
		if !to.After(from) {
			return true, nil
		}
		var n int
		err := tx.QueryRow(`SELECT COUNT(*) FROM sessions
			WHERE id != ? AND id != ?
			  AND datetime(timestamp) < datetime(?)
			  AND datetime(timestamp, '+' || seconds || ' seconds') > datetime(?)`,
			ignoreA, ignoreB, to.Format(layout), from.Format(layout)).Scan(&n)
		return n == 0, err
	}

	// Successiva: la più vicina che inizia tra la fine di questa e fine + intervallo.
	end := start.Add(time.Duration(seconds) * time.Second)
	var nextID int64
	var nextSeconds int
	var nextTimestamp string
	err = tx.QueryRow(`SELECT id, seconds, timestamp FROM sessions WHERE `+sameKind+`
		AND datetime(timestamp) >= datetime(?) AND datetime(timestamp) <= datetime(?)
		ORDER BY datetime(timestamp) ASC LIMIT 1`,
		sessionID, projectID, activityType, sessionType, end.Format(layout), end.Add(span).Format(layout)).
		Scan(&nextID, &nextSeconds, &nextTimestamp)
	if err == nil {
		nextStart, perr := parseNaiveTimestamp(nextTimestamp)
		if perr != nil {
			return perr
		}
		free, ferr := gapIsFree(end, nextStart, sessionID, nextID)
		if ferr != nil {
			return fmt.Errorf("errore verifica intervallo tra sessioni: %v", ferr)
		}
		if free {
			if _, err := tx.Exec(`DELETE FROM sessions WHERE id = ?`, nextID); err != nil {
				return fmt.Errorf("errore unione sessioni: %v", err)
			}
			seconds = int(nextStart.Add(time.Duration(nextSeconds) * time.Second).Sub(start).Seconds())
			if _, err := tx.Exec(`UPDATE sessions SET seconds = ? WHERE id = ?`, seconds, sessionID); err != nil {
				return fmt.Errorf("errore unione sessioni: %v", err)
			}
			end = start.Add(time.Duration(seconds) * time.Second)
		}
	} else if err != sql.ErrNoRows {
		return fmt.Errorf("errore ricerca sessione successiva: %v", err)
	}

	// Precedente: la più vicina che finisce tra (inizio - intervallo) e l'inizio di questa.
	var prevID int64
	var prevSeconds int
	var prevTimestamp string
	err = tx.QueryRow(`SELECT id, seconds, timestamp FROM sessions WHERE `+sameKind+`
		AND datetime(timestamp, '+' || seconds || ' seconds') >= datetime(?)
		AND datetime(timestamp, '+' || seconds || ' seconds') <= datetime(?)
		ORDER BY datetime(timestamp, '+' || seconds || ' seconds') DESC LIMIT 1`,
		sessionID, projectID, activityType, sessionType, start.Add(-span).Format(layout), start.Format(layout)).
		Scan(&prevID, &prevSeconds, &prevTimestamp)
	if err == nil {
		prevStart, perr := parseNaiveTimestamp(prevTimestamp)
		if perr != nil {
			return perr
		}
		prevEnd := prevStart.Add(time.Duration(prevSeconds) * time.Second)
		free, ferr := gapIsFree(prevEnd, start, sessionID, prevID)
		if ferr != nil {
			return fmt.Errorf("errore verifica intervallo tra sessioni: %v", ferr)
		}
		if free {
			if _, err := tx.Exec(`DELETE FROM sessions WHERE id = ?`, sessionID); err != nil {
				return fmt.Errorf("errore unione sessioni: %v", err)
			}
			merged := int(end.Sub(prevStart).Seconds())
			if _, err := tx.Exec(`UPDATE sessions SET seconds = ? WHERE id = ?`, merged, prevID); err != nil {
				return fmt.Errorf("errore unione sessioni: %v", err)
			}
		}
	} else if err != sql.ErrNoRows {
		return fmt.Errorf("errore ricerca sessione precedente: %v", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("errore conferma unione sessioni: %v", err)
	}
	return nil
}

// unisciDopoScrittura esegue l'unione dopo che la scrittura principale è già confermata:
// un'unione fallita non deve mai far perdere o annullare il salvataggio dell'utente.
func unisciDopoScrittura(db *sql.DB, sessionIDs ...int64) {
	for _, id := range sessionIDs {
		if err := UnisciSessioniAdiacenti(db, id); err != nil {
			fmt.Printf("[DB] Unione sessioni adiacenti non riuscita (ID %d): %v\n", id, err)
		}
	}
}
