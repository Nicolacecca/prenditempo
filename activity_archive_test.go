package main

import (
	"path/filepath"
	"testing"

	"work-time-tracker-go/tracker"
)

func TestArchivedActivityTypeRejectedForNewSessionsAndSurvivesBackup(t *testing.T) {
	db, err := tracker.InitDB(filepath.Join(t.TempDir(), "archive.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app := NewApp()
	app.SetDB(db)

	id, _ := tracker.CreaTipoAttivita(db, "Ritirato", 0, "solid", 90)
	name := "Ritirato"
	if err := app.CreateSession("S", 600, nil, "manual", &name, "2026-08-19 09:00:00"); err != nil {
		t.Fatal(err)
	}
	if archived, err := app.DeleteActivityType(int(id)); err != nil || !archived {
		t.Fatalf("atteso archiviato, archived=%v err=%v", archived, err)
	}
	if err := app.CreateSession("S2", 600, nil, "manual", &name, "2026-08-19 10:00:00"); err == nil {
		t.Fatal("un tipo archiviato non deve essere assegnabile a una nuova sessione")
	}

	backup, err := app.ExportData()
	if err != nil {
		t.Fatal(err)
	}
	if err := app.ImportData(*backup); err != nil {
		t.Fatal(err)
	}
	var archived int
	if err := db.QueryRow(`SELECT archived FROM activity_types WHERE name = 'Ritirato'`).Scan(&archived); err != nil || archived != 1 {
		t.Fatalf("il flag archiviato deve sopravvivere a export/import, archived=%d err=%v", archived, err)
	}
}
