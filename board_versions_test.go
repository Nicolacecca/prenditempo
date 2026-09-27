package main

import "testing"

func TestBoardVersionsAutomaticNamedRestoreAndDelete(t *testing.T) {
	app, cleanup := newTestApp(t)
	defer cleanup()

	projectID, err := app.CreateProject("Versioni lavagna", "")
	if err != nil {
		t.Fatal(err)
	}
	first := ProjectBoardData{
		ProjectID: int(projectID), SchemaVersion: currentBoardSchemaVersion,
		DocumentJSON: `{"objects":[{"type":"text","text":"Prima"}]}`,
		ViewportJSON: `{}`, LegacyNoteImported: true,
	}
	if err := app.SaveProjectBoard(first); err != nil {
		t.Fatal(err)
	}
	second := first
	second.DocumentJSON = `{"objects":[{"type":"text","text":"Seconda"}]}`
	if err := app.SaveProjectBoard(second); err != nil {
		t.Fatal(err)
	}

	versions, err := app.ListBoardVersions(int(projectID))
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 1 || !versions[0].IsAutomatic {
		t.Fatalf("attesa una versione automatica, ottenuto %#v", versions)
	}
	restored, err := app.GetBoardVersion(int(projectID), versions[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if restored.DocumentJSON != first.DocumentJSON {
		t.Fatalf("versione automatica errata: %s", restored.DocumentJSON)
	}

	named, err := app.CreateBoardVersion(int(projectID), "Prima della revisione")
	if err != nil {
		t.Fatal(err)
	}
	if named.IsAutomatic || named.Name != "Prima della revisione" {
		t.Fatalf("versione nominata incoerente: %#v", named)
	}
	third := second
	third.DocumentJSON = `{"objects":[{"type":"text","text":"Terza"}]}`
	if err := app.SaveProjectBoard(third); err != nil {
		t.Fatal(err)
	}
	if _, err := app.RestoreBoardVersion(int(projectID), versions[0].ID); err != nil {
		t.Fatal(err)
	}
	current, err := app.GetProjectBoard(int(projectID))
	if err != nil {
		t.Fatal(err)
	}
	if current.DocumentJSON != first.DocumentJSON {
		t.Fatalf("ripristino non applicato: %s", current.DocumentJSON)
	}
	if err := app.DeleteBoardVersion(int(projectID), named.ID); err != nil {
		t.Fatal(err)
	}
	versions, err = app.ListBoardVersions(int(projectID))
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 2 {
		t.Fatalf("la versione nominata non è stata eliminata: %#v", versions)
	}
	foundSafetyCopy := false
	for _, version := range versions {
		if version.IsAutomatic && version.Name == "Prima del ripristino" {
			foundSafetyCopy = true
		}
	}
	if !foundSafetyCopy {
		t.Fatal("il ripristino non ha creato la copia di sicurezza preventiva")
	}
}

func TestCreateBoardVersionValidatesName(t *testing.T) {
	app, cleanup := newTestApp(t)
	defer cleanup()
	projectID, err := app.CreateProject("Versioni", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.CreateBoardVersion(int(projectID), "   "); err == nil {
		t.Fatal("atteso rifiuto del nome vuoto")
	}
}
