package main

import (
	"crypto/sha256"
	"encoding/hex"
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

	result, err := db.Exec(`INSERT INTO projects (name, description, note_text) VALUES (?, ?, ?)`, "Progetto", "Descrizione", "# Nota")
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
	if err := tracker.SaveProjectBoard(db, tracker.ProjectBoard{
		ProjectID:          int(projectID),
		SchemaVersion:      1,
		DocumentJSON:       `{"objects":[{"type":"text"}]}`,
		ViewportJSON:       `{"viewportTransform":[1,0,0,1,0,0]}`,
		LegacyNoteImported: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tracker.CreateBoardVersion(db, tracker.BoardVersion{
		ProjectID: int(projectID), Name: "Versione prova",
		DocumentJSON: `{"objects":[{"type":"text","text":"storico"}]}`,
		ViewportJSON: `{}`,
	}); err != nil {
		t.Fatal(err)
	}
	imageData := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0}
	imageHash := sha256.Sum256(imageData)
	assetID := "0123456789abcdef0123456789abcdef"
	if _, err := db.Exec(`
		INSERT INTO board_assets (id, project_id, file_name, mime_type, data, size, sha256)
		VALUES (?, ?, 'pixel.png', 'image/png', ?, ?, ?)`,
		assetID, projectID, imageData, len(imageData), hex.EncodeToString(imageHash[:])); err != nil {
		t.Fatal(err)
	}

	InitGlobalState(db)
	app := NewApp()
	app.SetDB(db)
	backup, err := app.ExportData()
	if err != nil {
		t.Fatal(err)
	}
	if backup.Version != "2.3" || len(backup.Projects) != 1 {
		t.Fatalf("backup inatteso: versione=%s progetti=%d", backup.Version, len(backup.Projects))
	}
	if backup.Projects[0].NoteText != "# Nota" || len(backup.Settings) == 0 || len(backup.DayEvents) != 1 || len(backup.ProjectBoards) != 1 || len(backup.BoardAssets) != 1 || len(backup.BoardVersions) != 1 {
		t.Fatal("il backup non contiene tutti i dati correnti")
	}

	if _, err := db.Exec(`UPDATE projects SET note_text = 'modificata'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM day_events`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM board_assets`); err != nil {
		t.Fatal(err)
	}
	if err := tracker.SetSetting(db, "idle_threshold", "2"); err != nil {
		t.Fatal(err)
	}

	if err := app.ImportData(*backup); err != nil {
		t.Fatal(err)
	}
	var note string
	if err := db.QueryRow(`SELECT note_text FROM projects WHERE name = 'Progetto'`).Scan(&note); err != nil {
		t.Fatal(err)
	}
	if note != "# Nota" {
		t.Fatalf("nota ripristinata = %q", note)
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
	var boardDocument string
	if err := db.QueryRow(`SELECT document_json FROM project_boards`).Scan(&boardDocument); err != nil {
		t.Fatal(err)
	}
	if boardDocument != `{"objects":[{"type":"text"}]}` {
		t.Fatalf("lavagna ripristinata = %q", boardDocument)
	}
	var restoredImage []byte
	var restoredHash string
	if err := db.QueryRow(`SELECT data, sha256 FROM board_assets WHERE id = ?`, assetID).Scan(&restoredImage, &restoredHash); err != nil {
		t.Fatal(err)
	}
	if string(restoredImage) != string(imageData) || restoredHash != hex.EncodeToString(imageHash[:]) {
		t.Fatal("immagine della lavagna non ripristinata correttamente")
	}
	var restoredVersionName string
	if err := db.QueryRow(`SELECT name FROM board_versions`).Scan(&restoredVersionName); err != nil {
		t.Fatal(err)
	}
	if restoredVersionName != "Versione prova" {
		t.Fatalf("versione lavagna ripristinata = %q", restoredVersionName)
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
	InitGlobalState(db)
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
