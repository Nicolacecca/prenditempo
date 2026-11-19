package tracker

import (
	"path/filepath"
	"testing"
)

func TestRecoverPendingTrackingIsAtomicAndRepeatable(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "rec.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	pid64, _ := CreaProgetto(db, "P", "")
	pid := int(pid64)

	// Pendenti multipli (es. DB di una vecchia versione): uno con checkpoint, uno vuoto.
	withCheckpoint, _ := db.Exec(`INSERT INTO sessions (app_name, seconds, project_id, session_type, timestamp) VALUES ('A', 0, ?, 'computer', '2026-08-19 09:00:00')`, pid)
	idA, _ := withCheckpoint.LastInsertId()
	empty, _ := db.Exec(`INSERT INTO sessions (app_name, seconds, project_id, session_type, timestamp) VALUES ('B', 0, ?, 'computer', '2026-08-19 12:00:00')`, pid)
	idB, _ := empty.LastInsertId()
	db.Exec(`INSERT INTO pending_tracking (session_id, project_id, start_time, last_saved_seconds) VALUES (?, ?, '2026-08-19 09:00:00', 90)`, idA, pid)
	db.Exec(`INSERT INTO pending_tracking (session_id, project_id, start_time, last_saved_seconds) VALUES (?, ?, '2026-08-19 12:00:00', 0)`, idB, pid)

	n, err := RecoverPendingTracking(db)
	if err != nil || n != 2 {
		t.Fatalf("attesi 2 recuperi, n=%d err=%v", n, err)
	}
	var seconds, sessions, pending int
	db.QueryRow(`SELECT seconds FROM sessions WHERE id = ?`, idA).Scan(&seconds)
	db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&sessions)
	db.QueryRow(`SELECT COUNT(*) FROM pending_tracking`).Scan(&pending)
	if seconds != 90 || sessions != 1 || pending != 0 {
		t.Fatalf("stato dopo recupero: seconds=%d sessioni=%d pending=%d", seconds, sessions, pending)
	}

	// Ripetuto: idempotente, nulla da recuperare.
	if n, err := RecoverPendingTracking(db); err != nil || n != 0 {
		t.Fatalf("il secondo recupero deve essere vuoto, n=%d err=%v", n, err)
	}
}

func TestRecoverPendingTrackingRollsBackOnError(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "rec2.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	res, _ := db.Exec(`INSERT INTO sessions (app_name, seconds, session_type, timestamp) VALUES ('A', 0, 'computer', '2026-08-19 09:00:00')`)
	id, _ := res.LastInsertId()
	db.Exec(`INSERT INTO pending_tracking (session_id, start_time, last_saved_seconds) VALUES (?, '2026-08-19 09:00:00', 60)`, id)
	db.Exec(`CREATE TRIGGER fail_recovery BEFORE DELETE ON pending_tracking BEGIN SELECT RAISE(ABORT, 'boom'); END`)

	if n, _ := RecoverPendingTracking(db); n != 0 {
		t.Fatalf("nessun recupero atteso con errore, n=%d", n)
	}
	var seconds, pending int
	db.QueryRow(`SELECT seconds FROM sessions WHERE id = ?`, id).Scan(&seconds)
	db.QueryRow(`SELECT COUNT(*) FROM pending_tracking`).Scan(&pending)
	if seconds != 0 || pending != 1 {
		t.Fatalf("rollback atteso: seconds=%d pending=%d", seconds, pending)
	}
}
