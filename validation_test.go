package main

import "testing"

func TestParseClockStrict(t *testing.T) {
	for _, value := range []string{"8:00", "24:00", "12:60", " 08:00"} {
		if _, err := parseClock(value); err == nil {
			t.Fatalf("orario %q accettato", value)
		}
	}
	if minutes, err := parseClock("08:30"); err != nil || minutes != 510 {
		t.Fatalf("orario valido non riconosciuto: minuti=%d err=%v", minutes, err)
	}
}

func TestDaysInMonth(t *testing.T) {
	cases := map[int]int{1: 31, 2: 29, 4: 30, 12: 31}
	for month, want := range cases {
		if got := daysInMonth(month); got != want {
			t.Errorf("daysInMonth(%d) = %d, want %d", month, got, want)
		}
	}
}

func TestValidationUtilities(t *testing.T) {
	if got := neutralizeCSVFormula("=SUM(A1:A2)"); got != "'=SUM(A1:A2)" {
		t.Fatalf("formula CSV non neutralizzata: %q", got)
	}
	if got := neutralizeCSVFormula("testo"); got != "testo" {
		t.Fatalf("testo CSV alterato: %q", got)
	}
	if got := safeFilenamePart(`A<B>:C/D\E|F?G*`); got != "A_B__C_D_E_F_G_" {
		t.Fatalf("nome file inatteso: %q", got)
	}
	if _, err := parseLocalTimestamp("2026-08-19T09:30:00Z"); err != nil {
		t.Fatal(err)
	}
	if _, err := parseLocalTimestamp("19/08/2026 09:30"); err == nil {
		t.Fatal("timestamp ambiguo accettato")
	}
}

func TestComputeContractedHours(t *testing.T) {
	if hours := computeContractedHours("08:00", "17:00", true, "13:00", "14:00"); hours != 8 {
		t.Fatalf("ore contrattuali = %.2f", hours)
	}
	if hours := computeContractedHours("17:00", "08:00", false, "13:00", "14:00"); hours != 0 {
		t.Fatalf("orario invertito ha prodotto %.2f ore", hours)
	}
}
