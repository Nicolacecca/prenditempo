package tracker

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func newOverlapTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := InitDB(filepath.Join(t.TempDir(), "overlap-test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestSessionOverlapRejectedOnCreate(t *testing.T) {
	db := newOverlapTestDB(t)
	if err := CreaSessione(db, "Base", 3600, nil, "manual", nil, "2026-08-19 09:00:00"); err != nil {
		t.Fatal(err)
	}

	cases := map[string]string{
		"completa":  "2026-08-19 09:00:00", // stesso intervallo esatto
		"parziale":  "2026-08-19 09:30:00", // inizia dentro, finisce dopo
		"contenuta": "2026-08-19 09:15:00", // interamente dentro
	}
	for name, timestamp := range cases {
		t.Run(name, func(t *testing.T) {
			if err := CreaSessione(db, "Sovrapposta", 1800, nil, "manual", nil, timestamp); err == nil {
				t.Fatalf("sovrapposizione %s: attesa un errore", name)
			}
		})
	}

	// Sessione che contiene interamente quella esistente (09:00-10:00)
	if err := CreaSessione(db, "Contenitore", 7200, nil, "manual", nil, "2026-08-19 08:30:00"); err == nil {
		t.Fatal("sovrapposizione (contenitore): atteso un errore")
	}
}

func TestSessionAdjacentIntervalsAllowed(t *testing.T) {
	db := newOverlapTestDB(t)
	if err := CreaSessione(db, "Prima", 3600, nil, "manual", nil, "2026-08-19 09:00:00"); err != nil {
		t.Fatal(err)
	}
	// 10:00-11:00 è adiacente a 09:00-10:00: non deve essere rifiutata.
	if err := CreaSessione(db, "Dopo", 3600, nil, "off-computer", nil, "2026-08-19 10:00:00"); err != nil {
		t.Fatalf("intervalli adiacenti devono essere validi: %v", err)
	}
	// Adiacente anche prima: 08:00-09:00.
	if err := CreaSessione(db, "Prima ancora", 3600, nil, "off-computer", nil, "2026-08-19 08:00:00"); err != nil {
		t.Fatalf("intervalli adiacenti devono essere validi: %v", err)
	}
}

func TestSessionOverlapAcrossMidnight(t *testing.T) {
	db := newOverlapTestDB(t)
	// 23:00 del 19 -> 01:00 del 20 (2 ore)
	if err := CreaSessione(db, "Notturna", 7200, nil, "manual", nil, "2026-08-19 23:00:00"); err != nil {
		t.Fatal(err)
	}
	// 00:30 del 20 ricade dentro l'intervallo precedente.
	if err := CreaSessione(db, "Sovrapposta", 1800, nil, "manual", nil, "2026-08-20 00:30:00"); err == nil {
		t.Fatal("sovrapposizione oltre mezzanotte: atteso un errore")
	}
	// 01:00 del 20 è l'esatta fine della sessione precedente: adiacente, valida.
	if err := CreaSessione(db, "Successiva", 1800, nil, "off-computer", nil, "2026-08-20 01:00:00"); err != nil {
		t.Fatalf("intervallo adiacente oltre mezzanotte deve essere valido: %v", err)
	}
}

func TestSessionUpdateTimestampRejectsNewOverlap(t *testing.T) {
	db := newOverlapTestDB(t)
	if err := CreaSessione(db, "A", 3600, nil, "manual", nil, "2026-08-19 09:00:00"); err != nil { // 09:00-10:00
		t.Fatal(err)
	}
	if err := CreaSessione(db, "B", 3600, nil, "manual", nil, "2026-08-19 11:00:00"); err != nil { // 11:00-12:00
		t.Fatal(err)
	}
	var idB int
	if err := db.QueryRow(`SELECT id FROM sessions WHERE app_name = 'B'`).Scan(&idB); err != nil {
		t.Fatal(err)
	}

	// Spostare B a 09:30 (durata invariata: 09:30-10:30) la fa sovrapporre ad A (09:00-10:00).
	if err := AggiornaSessioneCompleta(db, idB, "2026-08-19 09:30:00", 3600, nil); err == nil {
		t.Fatal("update timestamp che crea sovrapposizione: atteso un errore")
	}
}

func TestSessionUpdateDurationRejectsNewOverlap(t *testing.T) {
	db := newOverlapTestDB(t)
	if err := CreaSessione(db, "A", 3600, nil, "manual", nil, "2026-08-19 09:00:00"); err != nil { // 09:00-10:00
		t.Fatal(err)
	}
	if err := CreaSessione(db, "D", 3600, nil, "off-computer", nil, "2026-08-19 10:00:00"); err != nil { // 10:00-11:00, adiacente
		t.Fatal(err)
	}
	var idA int
	if err := db.QueryRow(`SELECT id FROM sessions WHERE app_name = 'A'`).Scan(&idA); err != nil {
		t.Fatal(err)
	}

	// Allungare A a 2 ore (09:00-11:00) la fa sovrapporre a D (10:00-11:00).
	if err := AggiornaDurataSessione(db, idA, 7200); err == nil {
		t.Fatal("update durata che crea sovrapposizione: atteso un errore")
	}
}

func TestSessionSplitDoesNotFalsePositive(t *testing.T) {
	db := newOverlapTestDB(t)
	if err := CreaSessione(db, "Originale", 7200, nil, "manual", nil, "2026-08-19 09:00:00"); err != nil {
		t.Fatal(err)
	}
	var id int
	if err := db.QueryRow(`SELECT id FROM sessions WHERE app_name = 'Originale'`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	// Divide 09:00-11:00 in 09:00-10:00 e 10:00-11:00: non deve mai auto-collidere.
	if err := DividiSessione(db, id, 3600, strPtr("X"), strPtr("Y")); err != nil {
		t.Fatalf("split valido rifiutato per falso positivo: %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("attese 2 sessioni dopo lo split, trovate %d", count)
	}
}

func TestSessionOverlapBlocksImportBulkInsert(t *testing.T) {
	db := newOverlapTestDB(t)
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	insert := `INSERT INTO sessions (app_name, seconds, project_id, session_type, activity_type, timestamp) VALUES (?, ?, ?, ?, ?, ?)`
	if _, err := tx.Exec(insert, "Import1", 3600, nil, "manual", nil, "2026-08-19 09:00:00"); err != nil {
		t.Fatal(err)
	}
	// Sovrapposta alla precedente: deve far fallire lo statement e quindi l'intero import transazionale.
	if _, err := tx.Exec(insert, "Import2", 3600, nil, "manual", nil, "2026-08-19 09:30:00"); err == nil {
		t.Fatal("import con sessioni sovrapposte: atteso un errore che blocchi la transazione")
	}
}

func TestSessionOverlapConcurrentDoubleSubmit(t *testing.T) {
	db := newOverlapTestDB(t)
	if err := CreaSessione(db, "Prima", 3600, nil, "manual", nil, "2026-08-19 09:00:00"); err != nil {
		t.Fatal(err)
	}
	// Un secondo invio identico (doppio click) deve essere rifiutato come sovrapposizione,
	// non sommarsi silenziosamente al totale.
	if err := CreaSessione(db, "Prima", 3600, nil, "manual", nil, "2026-08-19 09:00:00"); err == nil {
		t.Fatal("doppio invio della stessa sessione: atteso un errore")
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("atteso 1 sola sessione dopo il doppio invio, trovate %d", count)
	}
}

func TestPendingTrackingGrowthIsNeverBlocked(t *testing.T) {
	db := newOverlapTestDB(t)
	pid64, _ := CreaProgetto(db, "P", "")
	pid := int(pid64)
	sid, err := StartPendingTracking(db, &pid, nil, "2026-08-19 13:00:00")
	if err != nil {
		t.Fatal(err)
	}
	// Sessione manuale che termina dopo l'inizio del tracking attivo: rifiutata.
	if err := CreaSessione(db, "Riunione", 3600, nil, "manual", nil, "2026-08-19 15:00:00"); err == nil {
		t.Fatal("attesa il rifiuto di una sessione che termina dopo l'inizio del tracking in corso")
	}
	// Sessione manuale conclusa prima dell'inizio del tracking: accettata.
	if err := CreaSessione(db, "Prima", 3600, nil, "manual", nil, "2026-08-19 11:00:00"); err != nil {
		t.Fatal(err)
	}
	// Checkpoint e finalizzazione non devono mai fallire.
	if err := UpdatePendingTracking(db, sid, 9000); err != nil {
		t.Fatalf("checkpoint bloccato: %v", err)
	}
	if err := FinalizePendingTracking(db, sid, 9000); err != nil {
		t.Fatalf("finalizzazione bloccata: %v", err)
	}
}

func TestLegacyOverlappingRowsRemainEditable(t *testing.T) {
	db := newOverlapTestDB(t)
	db.Exec(`DROP TRIGGER validate_sessions_no_overlap_insert`)
	db.Exec(`INSERT INTO sessions (app_name, seconds, session_type, timestamp) VALUES ('L1', 3600, 'manual', '2026-09-01 09:00:00')`)
	db.Exec(`INSERT INTO sessions (app_name, seconds, session_type, timestamp) VALUES ('L2', 3600, 'manual', '2026-09-01 09:30:00')`)
	var id int
	if err := db.QueryRow(`SELECT id FROM sessions WHERE app_name='L2'`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	activity := "RICERCA"
	if err := AggiornaActivityType(db, id, &activity); err != nil {
		t.Fatalf("modifica del solo tipo attività su riga legacy bloccata: %v", err)
	}
}

func strPtr(s string) *string { return &s }
