package main

import (
	"path/filepath"
	"testing"

	"work-time-tracker-go/tracker"
)

func TestStopTrackingFailureKeepsStateRecoverable(t *testing.T) {
	db, err := tracker.InitDB(filepath.Join(t.TempDir(), "stop.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app := NewApp()
	app.SetDB(db)

	pid64, _ := tracker.CreaProgetto(db, "P", "")
	if err := app.StartTracking(int(pid64), nil); err != nil {
		t.Fatal(err)
	}

	// La finalizzazione fallisce: lo stop non deve sembrare riuscito.
	db.Exec(`CREATE TRIGGER fail_finalize BEFORE DELETE ON pending_tracking BEGIN SELECT RAISE(ABORT, 'boom'); END`)
	if _, err := app.StopTracking(); err == nil {
		t.Fatal("atteso errore dallo stop con salvataggio fallito")
	}
	if !app.GetTrackingState().IsTracking {
		t.Fatal("dopo un salvataggio fallito il tracking deve restare attivo/recuperabile")
	}
	var pending int
	db.QueryRow(`SELECT COUNT(*) FROM pending_tracking`).Scan(&pending)
	if pending != 1 {
		t.Fatalf("la sessione pendente deve restare recuperabile, pending=%d", pending)
	}

	// Ripristinato il DB, il secondo stop va a buon fine.
	db.Exec(`DROP TRIGGER fail_finalize`)
	if _, err := app.StopTracking(); err != nil {
		t.Fatalf("il secondo stop deve riuscire: %v", err)
	}
	if app.GetTrackingState().IsTracking {
		t.Fatal("tracking ancora attivo dopo stop riuscito")
	}
}
