package main

import (
	"strings"
	"testing"
)

func TestProjectBoardRoundTripPreservesLegacyNote(t *testing.T) {
	app, cleanup := newTestApp(t)
	defer cleanup()

	projectID, err := app.CreateProject("Lavagna", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := app.UpdateProjectNote(int(projectID), "# Nota originale"); err != nil {
		t.Fatal(err)
	}

	board := ProjectBoardData{
		ProjectID:          int(projectID),
		SchemaVersion:      currentBoardSchemaVersion,
		DocumentJSON:       `{"objects":[{"type":"text","text":"Nota originale"}]}`,
		ViewportJSON:       `{"zoom":1,"x":0,"y":0}`,
		LegacyNoteImported: true,
	}
	if err := app.SaveProjectBoard(board); err != nil {
		t.Fatal(err)
	}
	loaded, err := app.GetProjectBoard(int(projectID))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.DocumentJSON != board.DocumentJSON || !loaded.LegacyNoteImported {
		t.Fatalf("lavagna riletta non coerente: %#v", loaded)
	}

	var note string
	if err := app.db.QueryRow(`SELECT note_text FROM projects WHERE id = ?`, projectID).Scan(&note); err != nil {
		t.Fatal(err)
	}
	if note != "# Nota originale" {
		t.Fatalf("la nota originale è stata modificata: %q", note)
	}
}

func TestSaveProjectBoardRejectsInvalidOrOversizedJSON(t *testing.T) {
	app, cleanup := newTestApp(t)
	defer cleanup()
	projectID, err := app.CreateProject("Limiti", "")
	if err != nil {
		t.Fatal(err)
	}

	board := ProjectBoardData{ProjectID: int(projectID), SchemaVersion: 1, DocumentJSON: "{", ViewportJSON: "{}"}
	if err := app.SaveProjectBoard(board); err == nil {
		t.Fatal("atteso rifiuto del JSON non valido")
	}
	board.DocumentJSON = `{"value":"` + strings.Repeat("x", maxBoardDocumentBytes) + `"}`
	if err := app.SaveProjectBoard(board); err == nil {
		t.Fatal("atteso rifiuto del documento troppo grande")
	}
}
