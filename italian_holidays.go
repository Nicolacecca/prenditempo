package main

import "time"

// italianHoliday è una festività nazionale con nome descrittivo.
type italianHoliday struct {
	Date time.Time
	Name string
}

// easterSunday calcola la data della Pasqua (calendario gregoriano) con
// l'algoritmo anonimo di Meeus/Jones/Butcher. Valido per qualunque anno del
// calendario gregoriano.
func easterSunday(year int) time.Time {
	a := year % 19
	b := year / 100
	c := year % 100
	d := b / 4
	e := b % 4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - d - g + 15) % 30
	i := c / 4
	k := c % 4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a + 11*h + 22*l) / 451
	month := (h + l - 7*m + 114) / 31
	day := (h+l-7*m+114)%31 + 1
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local)
}

// italianNationalHolidays elenca le festività nazionali italiane per l'anno
// indicato. Il 4 novembre (Unità Nazionale) NON è incluso: è una solennità
// civile ma non un giorno festivo lavorativo dal 1977 (L. 54/1977).
// Il 4 ottobre (San Francesco d'Assisi) è incluso solo dal 2026, anno di
// decorrenza della L. 8 ottobre 2025 n. 151 che l'ha reintrodotto.
func italianNationalHolidays(year int) []italianHoliday {
	easter := easterSunday(year)
	holidays := []italianHoliday{
		{time.Date(year, 1, 1, 0, 0, 0, 0, time.Local), "Capodanno"},
		{time.Date(year, 1, 6, 0, 0, 0, 0, time.Local), "Epifania"},
		{easter, "Pasqua"},
		{easter.AddDate(0, 0, 1), "Lunedì dell'Angelo"},
		{time.Date(year, 4, 25, 0, 0, 0, 0, time.Local), "Festa della Liberazione"},
		{time.Date(year, 5, 1, 0, 0, 0, 0, time.Local), "Festa del Lavoro"},
		{time.Date(year, 6, 2, 0, 0, 0, 0, time.Local), "Festa della Repubblica"},
		{time.Date(year, 8, 15, 0, 0, 0, 0, time.Local), "Ferragosto"},
		{time.Date(year, 11, 1, 0, 0, 0, 0, time.Local), "Ognissanti"},
		{time.Date(year, 12, 8, 0, 0, 0, 0, time.Local), "Immacolata Concezione"},
		{time.Date(year, 12, 25, 0, 0, 0, 0, time.Local), "Natale"},
		{time.Date(year, 12, 26, 0, 0, 0, 0, time.Local), "Santo Stefano"},
	}
	if year >= 2026 {
		holidays = append(holidays, italianHoliday{time.Date(year, 10, 4, 0, 0, 0, 0, time.Local), "San Francesco d'Assisi"})
	}
	return holidays
}
