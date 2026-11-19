package tracker

import (
	"fmt"
	"strings"
	"time"
)

// TimestampLayout è il formato con cui sessioni, note e progetti sono salvati.
const TimestampLayout = "2006-01-02 15:04:05"

// CONVENZIONE TEMPORALE
//
// Ogni timestamp è un'ORA LOCALE (orologio a muro) senza fuso orario, nel formato
// "YYYY-MM-DD HH:MM:SS". Il driver SQLite lo restituisce come "YYYY-MM-DDTHH:MM:SSZ":
// quella "Z" è solo un'etichetta del driver, NON indica UTC, e per questo viene
// accettata e ignorata. Qualsiasi altro suffisso (+02:00, -05:00, frazioni di secondo)
// viene rifiutato invece di essere troncato in silenzio, perché cambierebbe significato.
//
// Cambio ora legale (CET/CEST): un orario inesistente (02:30 del giorno in cui si
// avanza l'orologio) viene rifiutato in input. Un orario ambiguo (02:30 del giorno in cui
// si arretra) è accettato e interpretato alla prima occorrenza, perché il testo non
// permette di distinguere. Il report calcola le durate in tempo reale; trigger e split
// ragionano sul testo, quindi nella sola ora di cambio possono differire di un'ora.

// normalizeTimestamp applica la convenzione e restituisce il testo "YYYY-MM-DD HH:MM:SS".
func normalizeTimestamp(value string) (string, error) {
	clean := strings.Replace(strings.TrimSpace(value), "T", " ", 1)
	if len(clean) > 19 {
		suffix := clean[19:]
		if suffix != "Z" {
			return "", fmt.Errorf("timestamp con fuso orario o frazioni di secondo non supportato (%q): usa l'ora locale nel formato AAAA-MM-GG HH:MM:SS", value)
		}
		clean = clean[:19]
	}
	return clean, nil
}

// ParseLocalTimestamp interpreta un timestamp come ora locale, rifiutando orari
// con offset e orari inesistenti per il cambio ora legale.
func ParseLocalTimestamp(value string) (time.Time, error) {
	clean, err := normalizeTimestamp(value)
	if err != nil {
		return time.Time{}, err
	}
	t, err := time.ParseInLocation(TimestampLayout, clean, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("timestamp non valido: %s", value)
	}
	if t.Format(TimestampLayout) != clean {
		return time.Time{}, fmt.Errorf("orario inesistente per il cambio ora legale: %s", value)
	}
	return t, nil
}

// parseNaiveTimestamp legge il testo senza fuso orario, per l'aritmetica sul testo
// (split di sessioni) coerente con i trigger SQL.
func parseNaiveTimestamp(value string) (time.Time, error) {
	clean, err := normalizeTimestamp(value)
	if err != nil {
		return time.Time{}, err
	}
	t, err := time.Parse(TimestampLayout, clean)
	if err != nil {
		return time.Time{}, fmt.Errorf("timestamp non valido: %s", value)
	}
	return t, nil
}
