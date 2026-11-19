package tracker

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// useRome imposta il fuso su Europe/Rome per i test di cambio ora legale.
func useRome(t *testing.T) {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Rome")
	if err != nil {
		t.Skipf("database fusi orari non disponibile: %v", err)
	}
	old := time.Local
	time.Local = loc
	t.Cleanup(func() { time.Local = old })
}

func TestParseLocalTimestampAcceptedForms(t *testing.T) {
	useRome(t)
	for _, in := range []string{
		"2026-08-19 09:30:15",  // locale senza offset
		"2026-08-19T09:30:15",  // separatore T
		"2026-08-19T09:30:15Z", // "Z" = etichetta del driver per l'ora locale
		"  2026-08-19 09:30:15 ",
	} {
		got, err := ParseLocalTimestamp(in)
		if err != nil {
			t.Fatalf("%q rifiutato: %v", in, err)
		}
		if got.Format(TimestampLayout) != "2026-08-19 09:30:15" {
			t.Fatalf("%q interpretato come %s", in, got.Format(TimestampLayout))
		}
	}
}

func TestParseLocalTimestampRejectsOffsetsInsteadOfTruncating(t *testing.T) {
	useRome(t)
	for _, in := range []string{
		"2026-08-19 09:30:15+01:00",
		"2026-08-19T09:30:15+02:00",
		"2026-08-19T09:30:15-05:00",
		"2026-08-19T09:30:15.123Z",
		"2026-08-19 09:30:15 CEST",
	} {
		if _, err := ParseLocalTimestamp(in); err == nil {
			t.Fatalf("%q doveva essere rifiutato, non troncato", in)
		} else if !strings.Contains(err.Error(), "fuso orario") {
			t.Fatalf("%q: messaggio poco chiaro: %v", in, err)
		}
	}
	for _, in := range []string{"", "non una data", "2026-13-40 25:61:61"} {
		if _, err := ParseLocalTimestamp(in); err == nil {
			t.Fatalf("%q doveva essere rifiutato", in)
		}
	}
}

func TestParseLocalTimestampDaylightSavingTransitions(t *testing.T) {
	useRome(t)
	// Primavera 2026: alle 02:00 del 29 marzo l'orologio salta a 03:00.
	if _, err := ParseLocalTimestamp("2026-03-29 02:30:00"); err == nil {
		t.Fatal("un orario inesistente (salto CET->CEST) doveva essere rifiutato")
	}
	for _, ok := range []string{"2026-03-29 01:59:59", "2026-03-29 03:00:00"} {
		if _, err := ParseLocalTimestamp(ok); err != nil {
			t.Fatalf("%q valido rifiutato: %v", ok, err)
		}
	}
	// Autunno 2026: il 25 ottobre 02:30 esiste due volte; è accettato (prima occorrenza).
	if _, err := ParseLocalTimestamp("2026-10-25 02:30:00"); err != nil {
		t.Fatalf("un orario ambiguo (ripetuto) va accettato: %v", err)
	}
}

func TestSessionAcrossMidnightMonthAndYearEnd(t *testing.T) {
	useRome(t)
	db, err := InitDB(filepath.Join(t.TempDir(), "ts.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// 31/12 23:00 per 2 ore -> termina l'1/1 01:00: attraversa mezzanotte, fine mese e fine anno.
	if err := CreaSessione(db, "Capodanno", 7200, nil, "manual", nil, "2025-12-31 23:00:00"); err != nil {
		t.Fatal(err)
	}
	start, _ := ParseLocalTimestamp("2025-12-31 23:00:00")
	end := start.Add(2 * time.Hour)
	if end.Format(TimestampLayout) != "2026-01-01 01:00:00" {
		t.Fatalf("fine sessione = %s", end.Format(TimestampLayout))
	}
	// Una sessione adiacente subito dopo la mezzanotte è valida, una sovrapposta no.
	if err := CreaSessione(db, "Dopo", 600, nil, "manual", nil, "2026-01-01 01:00:00"); err != nil {
		t.Fatalf("adiacente oltre fine anno: %v", err)
	}
	if err := CreaSessione(db, "Sovrapposta", 600, nil, "manual", nil, "2026-01-01 00:30:00"); err == nil {
		t.Fatal("sovrapposizione oltre fine anno doveva essere rifiutata")
	}
}

func TestSplitUsesTextArithmeticAndAcceptsDriverTimestamps(t *testing.T) {
	useRome(t)
	db, err := InitDB(filepath.Join(t.TempDir(), "split.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := CreaSessione(db, "S", 7200, nil, "manual", nil, "2026-08-19 09:00:00"); err != nil {
		t.Fatal(err)
	}
	var id int
	db.QueryRow(`SELECT id FROM sessions`).Scan(&id)
	if err := DividiSessione(db, id, 3600, nil, nil); err != nil {
		t.Fatal(err)
	}
	var second string
	db.QueryRow(`SELECT substr(timestamp, 1, 19) FROM sessions WHERE id != ?`, id).Scan(&second)
	if strings.ReplaceAll(second, "T", " ") != "2026-08-19 10:00:00" {
		t.Fatalf("la seconda parte inizia a %q", second)
	}
}
