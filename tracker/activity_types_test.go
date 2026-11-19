package tracker

import (
	"path/filepath"
	"testing"
)

func TestDeleteUsedActivityTypeArchivesAndKeepsHistory(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "at.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	typeID64, err := CreaTipoAttivita(db, "Consulenza", 0, "solid", 90)
	if err != nil {
		t.Fatal(err)
	}
	typeID := int(typeID64)
	name := "Consulenza"
	if err := CreaSessione(db, "S", 3600, nil, "manual", &name, "2026-08-19 09:00:00"); err != nil {
		t.Fatal(err)
	}

	archived, err := EliminaTipoAttivita(db, typeID)
	if err != nil || !archived {
		t.Fatalf("atteso archiviazione, archived=%v err=%v", archived, err)
	}

	var label string
	if err := db.QueryRow(`SELECT activity_type FROM sessions WHERE app_name = 'S'`).Scan(&label); err != nil || label != "Consulenza" {
		t.Fatalf("lo storico deve conservare l'etichetta, trovato %q err=%v", label, err)
	}
	active, _ := CaricaTipiAttivita(db)
	for _, at := range active {
		if at.Name == "Consulenza" {
			t.Fatal("un tipo archiviato non deve comparire tra quelli attivi")
		}
	}
	all, _ := CaricaTuttiTipiAttivita(db)
	found := false
	for _, at := range all {
		if at.Name == "Consulenza" && at.Archived {
			found = true
		}
	}
	if !found {
		t.Fatal("il tipo archiviato deve restare disponibile per lo storico")
	}

	// Ricreare lo stesso nome riattiva il tipo (UNIQUE) invece di fallire.
	reID, err := CreaTipoAttivita(db, "Consulenza", 0.5, "dots", 91)
	if err != nil || int(reID) != typeID {
		t.Fatalf("attesa riattivazione dello stesso tipo, id=%d err=%v", reID, err)
	}
}

func TestDeleteUnusedActivityTypeRemovesIt(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "at2.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	id, _ := CreaTipoAttivita(db, "Mai usato", 0, "solid", 90)
	archived, err := EliminaTipoAttivita(db, int(id))
	if err != nil || archived {
		t.Fatalf("atteso delete reale, archived=%v err=%v", archived, err)
	}
	var n int
	db.QueryRow(`SELECT COUNT(*) FROM activity_types WHERE name = 'Mai usato'`).Scan(&n)
	if n != 0 {
		t.Fatal("il tipo mai usato doveva essere eliminato")
	}
}

func TestRenameActivityTypeUpdatesHistoryAtomically(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "at3.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	id64, _ := CreaTipoAttivita(db, "Vecchio", 0, "solid", 90)
	old := "Vecchio"
	if err := CreaSessione(db, "S", 600, nil, "manual", &old, "2026-08-19 09:00:00"); err != nil {
		t.Fatal(err)
	}
	if err := AggiornaTipoAttivita(db, int(id64), "Nuovo", 0, "solid", 90); err != nil {
		t.Fatal(err)
	}
	var label string
	db.QueryRow(`SELECT activity_type FROM sessions WHERE app_name = 'S'`).Scan(&label)
	if label != "Nuovo" {
		t.Fatalf("la rinomina deve aggiornare lo storico, trovato %q", label)
	}

	// Nome già usato da un altro tipo: UNIQUE fallisce e nulla deve cambiare (rollback).
	other, _ := CreaTipoAttivita(db, "Altro", 0, "solid", 91)
	_ = other
	if err := AggiornaTipoAttivita(db, int(id64), "Altro", 0, "solid", 90); err == nil {
		t.Fatal("atteso errore per nome duplicato")
	}
	db.QueryRow(`SELECT activity_type FROM sessions WHERE app_name = 'S'`).Scan(&label)
	if label != "Nuovo" {
		t.Fatalf("rollback fallito: etichetta storica = %q", label)
	}
}
