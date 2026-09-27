package main

import (
	"testing"
	"time"
)

func TestEasterSundayKnownDates(t *testing.T) {
	cases := map[int]string{
		2024: "2024-03-31",
		2025: "2025-04-20",
		2026: "2026-04-05",
		2027: "2027-03-28",
	}
	for year, want := range cases {
		got := easterSunday(year).Format("2006-01-02")
		if got != want {
			t.Errorf("easterSunday(%d) = %s, want %s", year, got, want)
		}
	}
}

func TestItalianNationalHolidaysSanFrancescoCutoff(t *testing.T) {
	hasSanFrancesco := func(year int) bool {
		for _, h := range italianNationalHolidays(year) {
			if h.Date.Month() == time.October && h.Date.Day() == 4 {
				return true
			}
		}
		return false
	}
	if hasSanFrancesco(2025) {
		t.Error("San Francesco d'Assisi non deve comparire prima del 2026")
	}
	if !hasSanFrancesco(2026) {
		t.Error("San Francesco d'Assisi deve comparire dal 2026")
	}
}

func TestItalianNationalHolidaysExcludes4Novembre(t *testing.T) {
	for _, h := range italianNationalHolidays(2026) {
		if h.Date.Month() == time.November && h.Date.Day() == 4 {
			t.Error("il 4 novembre non è un giorno festivo lavorativo, non deve essere incluso")
		}
	}
}
