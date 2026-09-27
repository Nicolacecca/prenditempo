package tracker

import (
	"math"
	"path/filepath"
	"testing"
)

func TestPendingTrackingIsSingleAndAtomic(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "tracking-test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	projectID64, err := CreaProgetto(db, "Test", "")
	if err != nil {
		t.Fatal(err)
	}
	projectID := int(projectID64)
	sessionID, err := StartPendingTracking(db, &projectID, nil, "2026-08-19 09:00:00")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := StartPendingTracking(db, &projectID, nil, "2026-08-19 10:00:00"); err == nil {
		t.Fatal("atteso rifiuto del secondo tracking pendente")
	}
	if err := UpdatePendingTracking(db, sessionID, 125); err != nil {
		t.Fatal(err)
	}
	if err := FinalizePendingTracking(db, sessionID, 180); err != nil {
		t.Fatal(err)
	}

	var seconds, pendingCount int
	if err := db.QueryRow(`SELECT seconds FROM sessions WHERE id = ?`, sessionID).Scan(&seconds); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM pending_tracking`).Scan(&pendingCount); err != nil {
		t.Fatal(err)
	}
	if seconds != 180 || pendingCount != 0 {
		t.Fatalf("stato finale non coerente: seconds=%d pending=%d", seconds, pendingCount)
	}
}

func TestFinalizeEmptyPendingTrackingRemovesPlaceholder(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "empty-tracking.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	sessionID, err := StartPendingTracking(db, nil, nil, "2026-08-19 09:00:00")
	if err != nil {
		t.Fatal(err)
	}
	if err := FinalizePendingTracking(db, sessionID, 0); err != nil {
		t.Fatal(err)
	}
	var sessions, pending int
	_ = db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&sessions)
	_ = db.QueryRow(`SELECT COUNT(*) FROM pending_tracking`).Scan(&pending)
	if sessions != 0 || pending != 0 {
		t.Fatalf("placeholder non rimosso: sessioni=%d pending=%d", sessions, pending)
	}
}

func TestMonthlyReportSplitsSessionsAtMonthBoundaries(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "monthly-boundaries.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	projectID64, err := CreaProgetto(db, "Confini", "")
	if err != nil {
		t.Fatal(err)
	}
	projectID := int(projectID64)
	for _, session := range []struct {
		timestamp string
		seconds   int
	}{
		{"2026-07-31 23:30:00", 3600},
		{"2026-08-31 23:30:00", 3600},
	} {
		if err := CreaSessione(db, "Sessione", session.seconds, &projectID, "computer", nil, session.timestamp); err != nil {
			t.Fatal(err)
		}
	}

	report, err := GetMonthlyReport(db, 2026, 8, 8, DefaultWorkingDaysBitmask, WorkScheduleConfig{StartMinutes: 8 * 60, EndMinutes: 17 * 60})
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(report.TotalTrackedHours-1) > 0.0001 {
		t.Fatalf("ore nel mese = %.3f, attese 1.000", report.TotalTrackedHours)
	}
	if math.Abs(report.Projects[0].TotalHours-1) > 0.0001 {
		t.Fatalf("ore progetto nel mese = %.3f", report.Projects[0].TotalHours)
	}
}

func TestMonthlyReportTreatsWeekendAndHolidayWorkAsOvertime(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "monthly-overtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	projectID64, _ := CreaProgetto(db, "Straordinari", "")
	projectID := int(projectID64)
	if err := CreaSessione(db, "Weekend", 3600, &projectID, "computer", nil, "2026-08-01 09:00:00"); err != nil {
		t.Fatal(err)
	}
	if err := SetDayEvent(db, "2026-08-19", "holiday", 0); err != nil {
		t.Fatal(err)
	}
	if err := CreaSessione(db, "Festivo", 3600, &projectID, "computer", nil, "2026-08-19 09:00:00"); err != nil {
		t.Fatal(err)
	}

	report, err := GetMonthlyReport(db, 2026, 8, 8, DefaultWorkingDaysBitmask, WorkScheduleConfig{StartMinutes: 8 * 60, EndMinutes: 17 * 60})
	if err != nil {
		t.Fatal(err)
	}
	byDate := make(map[string]DailyStats)
	for _, day := range report.Days {
		byDate[day.Date] = day
	}
	for _, date := range []string{"2026-08-01", "2026-08-19"} {
		if math.Abs(byDate[date].OutsideWorkHours-1) > 0.0001 || math.Abs(byDate[date].EffectiveWorkedHours-1) > 0.0001 {
			t.Fatalf("%s non conteggiato come straordinario: %+v", date, byDate[date])
		}
	}
}

func TestMonthlyReportKeepsContractForLeaveAndReducesOnlyHolidays(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "monthly-leave.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, event := range []struct {
		date      string
		eventType string
		hours     float64
	}{
		{"2026-08-16", "holiday", 0}, // domenica: non deve ridurre il contratto
		{"2026-08-17", "vacation", 0},
		{"2026-08-18", "permesso", 2},
		{"2026-08-19", "holiday", 0},
	} {
		if err := SetDayEvent(db, event.date, event.eventType, event.hours); err != nil {
			t.Fatal(err)
		}
	}

	report, err := GetMonthlyReport(db, 2026, 8, 8, DefaultWorkingDaysBitmask, WorkScheduleConfig{StartMinutes: 8 * 60, EndMinutes: 17 * 60})
	if err != nil {
		t.Fatal(err)
	}
	assertHours := func(label string, actual, expected float64) {
		t.Helper()
		if math.Abs(actual-expected) > 0.0001 {
			t.Fatalf("%s = %.3f, atteso %.3f", label, actual, expected)
		}
	}

	assertHours("ore contrattuali previste", report.TotalContractedHours, 168)
	assertHours("ore contrattuali effettive", report.EffectiveContractedHours, 160)
	assertHours("riduzione festività", report.HolidayHours, 8)
	assertHours("ore ferie", report.VacationHours, 8)
	assertHours("ore permesso", report.PermissionHours, 2)
	assertHours("giorni equivalenti di permesso", report.PermissionDays, 0.25)
	assertHours("saldo straordinari", report.OvertimeHours, 0)
	if report.HolidayDays != 1 || report.VacationDays != 1 {
		t.Fatalf("conteggi giorni errati: festività=%d ferie=%d", report.HolidayDays, report.VacationDays)
	}
}

func TestSplitSessionRollsBackIfSecondInsertFails(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "split-rollback.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := CreaSessione(db, "Sessione", 3600, nil, "computer", nil, "2026-08-19 09:00:00"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER fail_split BEFORE INSERT ON sessions BEGIN SELECT RAISE(ABORT, 'test'); END`); err != nil {
		t.Fatal(err)
	}
	if err := DividiSessione(db, 1, 1200, nil, nil); err == nil {
		t.Fatal("atteso errore durante la seconda INSERT")
	}
	var seconds, count int
	if err := db.QueryRow(`SELECT seconds FROM sessions WHERE id = 1`).Scan(&seconds); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if seconds != 3600 || count != 1 {
		t.Fatalf("rollback incompleto: durata=%d sessioni=%d", seconds, count)
	}
}

func TestDatabaseRejectsInvalidSessionData(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "constraints.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO sessions (app_name, seconds, session_type) VALUES ('', -1, 'invalid')`); err == nil {
		t.Fatal("atteso rifiuto dei dati sessione non validi")
	}
}

func TestForeignKeysAreEnabled(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "foreign-keys-test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(`INSERT INTO sessions (app_name, seconds, project_id) VALUES ('x', 1, 9999)`); err == nil {
		t.Fatal("atteso errore di foreign key")
	}
}

func TestMonthlyReportWorksWithConfiguredConnection(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "monthly-report-test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	projectID64, err := CreaProgetto(db, "Report", "")
	if err != nil {
		t.Fatal(err)
	}
	projectID := int(projectID64)
	if err := CreaSessione(db, "Sessione", 3600, &projectID, "computer", nil, "2026-08-19 09:00:00"); err != nil {
		t.Fatal(err)
	}
	report, err := GetMonthlyReport(db, 2026, 8, 8, DefaultWorkingDaysBitmask, WorkScheduleConfig{
		StartMinutes: 8 * 60,
		EndMinutes:   17 * 60,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.TotalTrackedHours != 1 || len(report.Projects) != 1 {
		t.Fatalf("report inatteso: ore=%.2f progetti=%d", report.TotalTrackedHours, len(report.Projects))
	}
}
