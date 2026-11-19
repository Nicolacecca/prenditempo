package tracker

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func mergeTestDB(t *testing.T) (*sql.DB, int) {
	t.Helper()
	db, err := InitDB(filepath.Join(t.TempDir(), "merge.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	id, err := CreaProgetto(db, "P", "")
	if err != nil {
		t.Fatal(err)
	}
	return db, int(id)
}

func sessionRows(t *testing.T, db *sql.DB) (count, totalSeconds int) {
	t.Helper()
	if err := db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(seconds), 0) FROM sessions`).Scan(&count, &totalSeconds); err != nil {
		t.Fatal(err)
	}
	return
}

func TestAdjacentSessionsSameProjectAndTypeMerge(t *testing.T) {
	db, pid := mergeTestDB(t)
	typ := "RICERCA"
	CreaSessione(db, "A", 3600, &pid, "computer", &typ, "2026-08-19 09:00:00")
	CreaSessione(db, "B", 1800, &pid, "computer", &typ, "2026-08-19 10:00:00")

	n, total := sessionRows(t, db)
	if n != 1 || total != 5400 {
		t.Fatalf("attesa 1 sessione da 5400 s, trovate %d da %d s", n, total)
	}
	var start string
	db.QueryRow(`SELECT timestamp FROM sessions`).Scan(&start)
	if start[:19] != "2026-08-19 09:00:00"[:19] && start[:19] != "2026-08-19T09:00:00"[:19] {
		t.Fatalf("la sessione unita deve iniziare all'inizio della più antica, inizia a %q", start)
	}
}

func TestMergeWorksInBothDirectionsAndBridgesThreeSessions(t *testing.T) {
	db, pid := mergeTestDB(t)
	typ := "X"
	CreaSessione(db, "Prima", 3600, &pid, "computer", &typ, "2026-08-19 09:00:00")  // 09-10
	CreaSessione(db, "Ultima", 3600, &pid, "computer", &typ, "2026-08-19 11:00:00") // 11-12
	if n, _ := sessionRows(t, db); n != 2 {
		t.Fatalf("con un buco in mezzo restano 2 sessioni, trovate %d", n)
	}
	// La sessione centrale riempie esattamente il buco: le tre diventano una.
	CreaSessione(db, "Centro", 3600, &pid, "computer", &typ, "2026-08-19 10:00:00")
	n, total := sessionRows(t, db)
	if n != 1 || total != 3*3600 {
		t.Fatalf("attesa 1 sessione da %d s, trovate %d da %d s", 3*3600, n, total)
	}
}

func TestSessionsThatDifferOrAreNotContiguousStaySeparate(t *testing.T) {
	x, y := "X", "Y"
	cases := []struct {
		name      string
		otherProj bool
		activity  *string
		kind      string
		timestamp string
	}{
		{"tipo attività diverso", false, &y, "computer", "2026-08-19 10:00:00"},
		{"progetto diverso", true, &x, "computer", "2026-08-19 10:00:00"},
		{"tipo sessione diverso", false, &x, "off-computer", "2026-08-19 10:00:00"},
		{"intervallo oltre la tolleranza (300 s)", false, &x, "computer", "2026-08-19 10:05:00"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db, pid := mergeTestDB(t)
			SetSetting(db, MergeGapSettingKey, "60")
			other, _ := CreaProgetto(db, "Altro", "")
			candidateProject := pid
			if c.otherProj {
				candidateProject = int(other)
			}
			// Base 09:00-10:00 (progetto P, tipo X, computer) e candidata che inizia alle 10:00.
			CreaSessione(db, "base", 3600, &pid, "computer", &x, "2026-08-19 09:00:00")
			CreaSessione(db, "candidata", 600, &candidateProject, c.kind, c.activity, c.timestamp)
			if n, total := sessionRows(t, db); n != 2 || total != 4200 {
				t.Fatalf("nessuna unione attesa: %d sessioni, %d s", n, total)
			}
		})
	}
}

func TestSessionsWithoutProjectAndActivityMergeToo(t *testing.T) {
	db, _ := mergeTestDB(t)
	CreaSessione(db, "A", 600, nil, "manual", nil, "2026-08-19 09:00:00")
	CreaSessione(db, "B", 600, nil, "manual", nil, "2026-08-19 09:10:00")
	if n, total := sessionRows(t, db); n != 1 || total != 1200 {
		t.Fatalf("NULL e NULL devono essere considerati uguali: %d sessioni, %d s", n, total)
	}
}

func TestEditingActivityTypeCanCreateAMerge(t *testing.T) {
	db, pid := mergeTestDB(t)
	x, y := "X", "Y"
	CreaSessione(db, "A", 3600, &pid, "computer", &x, "2026-08-19 09:00:00")
	CreaSessione(db, "B", 1800, &pid, "computer", &y, "2026-08-19 10:00:00")
	if n, _ := sessionRows(t, db); n != 2 {
		t.Fatalf("tipi diversi: attese 2 sessioni, trovate %d", n)
	}
	var idB int
	db.QueryRow(`SELECT id FROM sessions WHERE app_name = 'B'`).Scan(&idB)
	if err := AggiornaActivityType(db, idB, &x); err != nil {
		t.Fatal(err)
	}
	if n, total := sessionRows(t, db); n != 1 || total != 5400 {
		t.Fatalf("dopo aver reso uguale il tipo attesa 1 sessione da 5400 s, trovate %d da %d s", n, total)
	}
}

func TestSplitWithSameTypeMergesBackButDifferentTypesStaySplit(t *testing.T) {
	db, pid := mergeTestDB(t)
	x, y := "X", "Y"
	CreaSessione(db, "S", 7200, &pid, "computer", &x, "2026-08-19 09:00:00")
	var id int
	db.QueryRow(`SELECT id FROM sessions`).Scan(&id)

	if err := DividiSessione(db, id, 3600, &x, &y); err != nil {
		t.Fatal(err)
	}
	if n, total := sessionRows(t, db); n != 2 || total != 7200 {
		t.Fatalf("split con tipi diversi: attese 2 sessioni, trovate %d da %d s", n, total)
	}
	// Riportare la seconda parte allo stesso tipo della prima le riunisce.
	var secondID int
	db.QueryRow(`SELECT id FROM sessions WHERE id != ?`, id).Scan(&secondID)
	AggiornaActivityType(db, secondID, &x)
	if n, total := sessionRows(t, db); n != 1 || total != 7200 {
		t.Fatalf("attesa 1 sessione da 7200 s, trovate %d da %d s", n, total)
	}
}

func TestRunningTrackingSessionIsNeverMerged(t *testing.T) {
	db, pid := mergeTestDB(t)
	x := "X"
	CreaSessione(db, "Manuale", 3600, &pid, "computer", &x, "2026-08-19 09:00:00")
	sid, err := StartPendingTracking(db, &pid, &x, "2026-08-19 10:00:00") // inizia dove finisce la manuale
	if err != nil {
		t.Fatal(err)
	}
	if err := UpdatePendingTracking(db, sid, 600); err != nil {
		t.Fatal(err)
	}
	if n, _ := sessionRows(t, db); n != 2 {
		t.Fatalf("il tracking in corso non deve fondersi: %d sessioni", n)
	}
	// Alla chiusura la sessione finalizzata si unisce alla precedente contigua.
	if err := FinalizePendingTracking(db, sid, 900); err != nil {
		t.Fatal(err)
	}
	if n, total := sessionRows(t, db); n != 1 || total != 3600+900 {
		t.Fatalf("dopo la finalizzazione attesa 1 sessione da %d s, trovate %d da %d s", 3600+900, n, total)
	}
}

func TestMergeGapToleranceAbsorbsTheGap(t *testing.T) {
	db, pid := mergeTestDB(t)
	SetSetting(db, MergeGapSettingKey, "60")
	x := "X"
	CreaSessione(db, "A", 3600, &pid, "computer", &x, "2026-08-19 09:00:00") // 09:00:00-10:00:00
	CreaSessione(db, "B", 1800, &pid, "computer", &x, "2026-08-19 10:00:45") // 45 s dopo: entro la tolleranza

	n, total := sessionRows(t, db)
	if n != 1 || total != 3600+45+1800 {
		t.Fatalf("attesa 1 sessione da %d s (intervallo di 45 s assorbito), trovate %d da %d s", 3600+45+1800, n, total)
	}
	// La sessione unita termina dove terminava la seconda: 10:30:45.
	var seconds int
	db.QueryRow(`SELECT seconds FROM sessions`).Scan(&seconds)
	if seconds != 3600+45+1800 {
		t.Fatalf("durata unita = %d", seconds)
	}
}

func TestMergeGapWorksFromTheLaterSessionToo(t *testing.T) {
	db, pid := mergeTestDB(t)
	SetSetting(db, MergeGapSettingKey, "60")
	x := "X"
	CreaSessione(db, "B", 1800, &pid, "computer", &x, "2026-08-19 10:00:45") // prima la successiva
	CreaSessione(db, "A", 3600, &pid, "computer", &x, "2026-08-19 09:00:00") // poi la precedente, con 45 s di scarto
	if n, total := sessionRows(t, db); n != 1 || total != 3600+45+1800 {
		t.Fatalf("attesa 1 sessione da %d s, trovate %d da %d s", 3600+45+1800, n, total)
	}
}

func TestMergeGapZeroRequiresExactContact(t *testing.T) {
	db, pid := mergeTestDB(t)
	SetSetting(db, MergeGapSettingKey, "0")
	x := "X"
	CreaSessione(db, "A", 3600, &pid, "computer", &x, "2026-08-19 09:00:00")
	CreaSessione(db, "B", 600, &pid, "computer", &x, "2026-08-19 10:00:01")
	if n, _ := sessionRows(t, db); n != 2 {
		t.Fatalf("con intervallo 0 un secondo di scarto non si unisce: %d sessioni", n)
	}
	CreaSessione(db, "C", 600, &pid, "computer", &x, "2026-08-19 10:10:01") // contatto esatto con B
	if n, total := sessionRows(t, db); n != 2 || total != 3600+1200 {
		t.Fatalf("B e C devono unirsi, A resta a parte: %d sessioni, %d s", n, total)
	}
}

func TestMergeSkipsWhenAnotherSessionSitsInTheGap(t *testing.T) {
	db, pid := mergeTestDB(t)
	SetSetting(db, MergeGapSettingKey, "60")
	x, y := "X", "Y"
	CreaSessione(db, "A", 3600, &pid, "computer", &x, "2026-08-19 09:00:00") // fino alle 10:00:00
	CreaSessione(db, "C", 10, &pid, "computer", &y, "2026-08-19 10:00:10")   // tipo diverso nel mezzo
	CreaSessione(db, "B", 600, &pid, "computer", &x, "2026-08-19 10:00:30")  // entro 60 s da A, ma C in mezzo
	if n, total := sessionRows(t, db); n != 3 || total != 3600+10+600 {
		t.Fatalf("A e B non si uniscono con C in mezzo: %d sessioni, %d s", n, total)
	}
}

func TestMergeGapSettingFallsBackToDefault(t *testing.T) {
	db, _ := mergeTestDB(t)
	if MergeGapSeconds(db) != DefaultMergeGapSeconds {
		t.Fatal("senza impostazione deve valere il default")
	}
	for _, bad := range []string{"-5", "abc", "99999"} {
		SetSetting(db, MergeGapSettingKey, bad)
		if MergeGapSeconds(db) != DefaultMergeGapSeconds {
			t.Fatalf("valore non valido %q: atteso il default", bad)
		}
	}
	SetSetting(db, MergeGapSettingKey, "120")
	if MergeGapSeconds(db) != 120 {
		t.Fatal("valore valido non letto")
	}
}
