package main

import (
	"os"
	"path/filepath"
	"testing"

	"work-time-tracker-go/tracker"
)

func TestBackupRoundTripPreservesCurrentData(t *testing.T) {
	db, err := tracker.InitDB(filepath.Join(t.TempDir(), "backup-test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	result, err := db.Exec(`INSERT INTO projects (name, description, note_text, board_url) VALUES (?, ?, ?, ?)`, "Progetto", "Descrizione", "# Nota", "https://excalidraw.com/#room=abc")
	if err != nil {
		t.Fatal(err)
	}
	projectID, _ := result.LastInsertId()
	if _, err := db.Exec(`INSERT INTO sessions (app_name, seconds, project_id, session_type, activity_type, timestamp) VALUES (?, ?, ?, ?, ?, ?)`,
		"Sessione", 3600, projectID, "computer", "RICERCA", "2026-08-19 09:00:00"); err != nil {
		t.Fatal(err)
	}
	if err := tracker.SetSetting(db, "idle_threshold", "9"); err != nil {
		t.Fatal(err)
	}
	if err := tracker.SetDayEvent(db, "2026-08-15", "holiday", 0); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	app.SetDB(db)
	backup, err := app.ExportData()
	if err != nil {
		t.Fatal(err)
	}
	if backup.Version != "2.4" || len(backup.Projects) != 1 {
		t.Fatalf("backup inatteso: versione=%s progetti=%d", backup.Version, len(backup.Projects))
	}
	if backup.Projects[0].NoteText != "# Nota" || backup.Projects[0].BoardURL != "https://excalidraw.com/#room=abc" || len(backup.Settings) == 0 || len(backup.DayEvents) != 1 {
		t.Fatal("il backup non contiene tutti i dati correnti")
	}

	if _, err := db.Exec(`UPDATE projects SET note_text = 'modificata', board_url = ''`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM day_events`); err != nil {
		t.Fatal(err)
	}
	if err := tracker.SetSetting(db, "idle_threshold", "2"); err != nil {
		t.Fatal(err)
	}

	if err := app.ImportData(*backup); err != nil {
		t.Fatal(err)
	}
	var note, boardURL string
	if err := db.QueryRow(`SELECT note_text, board_url FROM projects WHERE name = 'Progetto'`).Scan(&note, &boardURL); err != nil {
		t.Fatal(err)
	}
	if note != "# Nota" {
		t.Fatalf("nota ripristinata = %q", note)
	}
	if boardURL != "https://excalidraw.com/#room=abc" {
		t.Fatalf("URL lavagna ripristinato = %q", boardURL)
	}
	setting, err := tracker.GetSetting(db, "idle_threshold")
	if err != nil || setting != "9" {
		t.Fatalf("impostazione ripristinata = %q, err=%v", setting, err)
	}
	var eventCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM day_events WHERE date = '2026-08-15'`).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if eventCount != 1 {
		t.Fatalf("eventi ripristinati = %d", eventCount)
	}
}

func TestImportCreatesAutomaticBackup(t *testing.T) {
	tempDir := t.TempDir()
	db, err := tracker.InitDB(filepath.Join(tempDir, "automatic-backup.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := tracker.CreaProgetto(db, "Prima", ""); err != nil {
		t.Fatal(err)
	}
	app := NewApp()
	app.SetDB(db)
	data, err := app.ExportData()
	if err != nil {
		t.Fatal(err)
	}
	if err := app.ImportData(*data); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(tempDir, "backups", "pre-import-*.json"))
	if err != nil || len(files) != 1 {
		t.Fatalf("backup automatici trovati=%d, err=%v", len(files), err)
	}
	if info, err := os.Stat(files[0]); err != nil || info.Size() == 0 {
		t.Fatalf("backup automatico non valido: info=%v err=%v", info, err)
	}
}

func TestSetWorkScheduleIsAtomicAndValidated(t *testing.T) {
	db, err := tracker.InitDB(filepath.Join(t.TempDir(), "schedule.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app := NewApp()
	app.SetDB(db)
	if schedule := app.GetWorkSchedule(); !schedule.LunchBreakEnabled || schedule.ContractedHours != 8 {
		t.Fatalf("orario predefinito inatteso: %+v", schedule)
	}
	if err := app.SetWorkSchedule("17:00", "08:00", true, "13:00", "14:00"); err == nil {
		t.Fatal("atteso rifiuto di un orario invertito")
	}
	if _, err := db.Exec(`CREATE TRIGGER fail_schedule BEFORE INSERT ON settings WHEN NEW.key = 'lunch_break_end' BEGIN SELECT RAISE(ABORT, 'test'); END`); err != nil {
		t.Fatal(err)
	}
	if err := app.SetWorkSchedule("09:00", "18:00", true, "13:00", "14:00"); err == nil {
		t.Fatal("atteso errore transazionale")
	}
	if value, _ := tracker.GetSetting(db, "work_start_time"); value != "" {
		t.Fatalf("salvataggio parziale rilevato: %s", value)
	}
}

func TestValidateBackupRejectsBrokenReferences(t *testing.T) {
	projectID := 99
	data := ExportDataResult{
		Version: "2.0",
		Sessions: []ExportSession{{
			ID: 1, AppName: "Sessione", Seconds: 60, ProjectID: &projectID,
			SessionType: "computer", Timestamp: "2026-08-19 09:00:00",
		}},
	}
	if err := validateBackupData(data); err == nil {
		t.Fatal("atteso errore per riferimento a progetto inesistente")
	}
}

func TestValidateBackupRejectsTimestampsWithOffsetButAcceptsDriverZ(t *testing.T) {
	session := func(ts string) ExportDataResult {
		return ExportDataResult{Version: "2.4", Sessions: []ExportSession{{
			ID: 1, AppName: "S", Seconds: 60, SessionType: "manual", Timestamp: ts,
		}}}
	}
	if err := validateBackupData(session("2026-08-19T09:00:00Z")); err != nil {
		t.Fatalf("il formato prodotto dal driver (Z) deve restare importabile: %v", err)
	}
	for _, ts := range []string{"2026-08-19T09:00:00+02:00", "2026-08-19 09:00:00-05:00"} {
		if err := validateBackupData(session(ts)); err == nil {
			t.Fatalf("%q: atteso rifiuto di un offset che verrebbe interpretato male", ts)
		}
	}
}
