package main

import (
	"path/filepath"
	"testing"

	"work-time-tracker-go/tracker"
)

func newTestApp(t *testing.T) (*App, func()) {
	t.Helper()
	db, err := tracker.InitDB(filepath.Join(t.TempDir(), "app-flow.db"))
	if err != nil {
		t.Fatal(err)
	}
	InitGlobalState(db)
	app := NewApp()
	app.SetDB(db)
	return app, func() {
		InitGlobalState(nil)
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
	globalStateMu.Lock()
	globalIsTracking = true
	globalCurrentProject = &tracker.Project{ID: projectID, Name: "In uso"}
	globalStateMu.Unlock()
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
