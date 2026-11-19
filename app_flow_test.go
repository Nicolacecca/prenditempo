package main

import (
	"path/filepath"
	"testing"
	"time"

	"work-time-tracker-go/tracker"
)

func newTestApp(t *testing.T) (*App, func()) {
	t.Helper()
	db, err := tracker.InitDB(filepath.Join(t.TempDir(), "app-flow.db"))
	if err != nil {
		t.Fatal(err)
	}
	app := NewApp()
	app.SetDB(db)
	return app, func() {
		_ = db.Close()
	}
}

func TestTrackedProjectCannotBeArchivedOrDeleted(t *testing.T) {
	app, cleanup := newTestApp(t)
	defer cleanup()
	projectID64, err := app.CreateProject("In uso", "")
	if err != nil {
		t.Fatal(err)
	}
	projectID := int(projectID64)
	app.track.mu.Lock()
	app.track.isTracking = true
	app.track.currentProject = &tracker.Project{ID: projectID, Name: "In uso"}
	app.track.mu.Unlock()
	if err := app.ArchiveProject(projectID); err == nil {
		t.Fatal("progetto in uso archiviato")
	}
	if err := app.DeleteProject(projectID); err == nil {
		t.Fatal("progetto in uso eliminato")
	}
}

func TestCreateSessionValidatesProjectActivityAndTimestamp(t *testing.T) {
	app, cleanup := newTestApp(t)
	defer cleanup()
	projectID64, err := app.CreateProject("Sessioni", "")
	if err != nil {
		t.Fatal(err)
	}
	projectID := int(projectID64)
	missingActivity := "INESISTENTE"
	if err := app.CreateSession("Manuale", 60, &projectID, "computer", &missingActivity, "2026-08-19 09:00:00"); err == nil {
		t.Fatal("tipo attività inesistente accettato")
	}
	if err := app.CreateSession("Manuale", 60, &projectID, "computer", nil, "data non valida"); err == nil {
		t.Fatal("timestamp non valido accettato")
	}
	if err := app.CreateSession("Manuale", 60, &projectID, "computer", nil, "2026-08-19 09:00:00"); err != nil {
		t.Fatal(err)
	}
}

func TestTrackingStateIsPerAppInstance(t *testing.T) {
	first, cleanupFirst := newTestApp(t)
	defer cleanupFirst()
	second, cleanupSecond := newTestApp(t)
	defer cleanupSecond()

	first.track.mu.Lock()
	first.track.isTracking = true
	first.track.mu.Unlock()
	if second.GetTrackingState().IsTracking {
		t.Fatal("lo stato del tracking non deve essere condiviso tra istanze di App")
	}
	if err := first.SetIdleThreshold(17); err != nil {
		t.Fatal(err)
	}
	if first.GetIdleThreshold() != 17 || second.GetIdleThreshold() != defaultIdleThresholdMinutes {
		t.Fatalf("soglie: prima=%d seconda=%d", first.GetIdleThreshold(), second.GetIdleThreshold())
	}
}

func TestSessionsCannotEndInTheFuture(t *testing.T) {
	app, cleanup := newTestApp(t)
	defer cleanup()

	start := time.Now().Add(-30 * time.Minute).Format("2006-01-02 15:04:05")
	if err := app.CreateSession("Passata", 600, nil, "manual", nil, start); err != nil {
		t.Fatalf("sessione già conclusa rifiutata: %v", err)
	}
	// Un'ora nel futuro: rifiutata.
	future := time.Now().Add(time.Hour).Format("2006-01-02 15:04:05")
	if err := app.CreateSession("Futura", 600, nil, "manual", nil, future); err == nil {
		t.Fatal("una sessione nel futuro doveva essere rifiutata")
	}
	// Inizia nel passato ma termina nel futuro: rifiutata.
	if err := app.CreateSession("Lunga", 3*3600, nil, "manual", nil, time.Now().Add(-time.Hour).Format("2006-01-02 15:04:05")); err == nil {
		t.Fatal("una sessione che termina nel futuro doveva essere rifiutata")
	}

	sessions, err := app.GetSessions(time.Now().Format("2006-01-02"), time.Now().Format("2006-01-02"))
	if err != nil || len(sessions) == 0 {
		t.Fatalf("sessione passata non trovata: %v", err)
	}
	id := sessions[0].ID
	if err := app.UpdateSessionDuration(id, 6*3600); err == nil {
		t.Fatal("allungare una sessione fino al futuro doveva essere rifiutato")
	}
	if err := app.UpdateSessionComplete(id, future, 600, nil); err == nil {
		t.Fatal("spostare una sessione nel futuro doveva essere rifiutato")
	}
}
