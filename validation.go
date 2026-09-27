package main

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func (a *App) validateActivityReference(activityType *string) error {
	if activityType == nil {
		return nil
	}
	name := strings.TrimSpace(*activityType)
	if name == "" || len([]rune(name)) > 100 {
		return fmt.Errorf("tipo attività non valido")
	}
	var exists int
	if err := a.db.QueryRow("SELECT 1 FROM activity_types WHERE name = ?", name).Scan(&exists); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("tipo attività inesistente: %s", name)
		}
		return err
	}
	return nil
}

func validateActivityType(name string, colorVariant float64, pattern string, displayOrder int) error {
	if strings.TrimSpace(name) == "" || len([]rune(name)) > 100 {
		return fmt.Errorf("nome tipo attività non valido")
	}
	if colorVariant < -1 || colorVariant > 1 {
		return fmt.Errorf("variante colore non valida")
	}
	if pattern != "solid" && pattern != "stripes" && pattern != "dots" {
		return fmt.Errorf("motivo attività non valido")
	}
	if displayOrder < 0 {
		return fmt.Errorf("ordine attività non valido")
	}
	return nil
}

func parseLocalTimestamp(value string) (time.Time, error) {
	clean := strings.Replace(strings.TrimSpace(value), "T", " ", 1)
	if len(clean) > 19 && (clean[19] == 'Z' || clean[19] == '+' || clean[19] == '-') {
		clean = clean[:19]
	}
	t, err := time.ParseInLocation("2006-01-02 15:04:05", clean, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("timestamp non valido: %s", value)
	}
	return t, nil
}

func parseClock(value string) (int, error) {
	parsed, err := time.Parse("15:04", value)
	if err != nil || parsed.Format("15:04") != value {
		return 0, fmt.Errorf("orario non valido: %s", value)
	}
	return parsed.Hour()*60 + parsed.Minute(), nil
}

func parseTimeToMinutes(value string) int {
	minutes, _ := parseClock(value)
	return minutes
}

// computeContractedHours è duplicata in frontend/src/schedule.js (calculateContractedHours)
// per il preview live lato UI senza round-trip IPC: tieni le due formule allineate.
func computeContractedHours(startTime, endTime string, lunchEnabled bool, lunchStart, lunchEnd string) float64 {
	start, startErr := parseClock(startTime)
	end, endErr := parseClock(endTime)
	if startErr != nil || endErr != nil || end <= start {
		return 0
	}
	total := end - start
	if lunchEnabled {
		lunchStartMinutes, startErr := parseClock(lunchStart)
		lunchEndMinutes, endErr := parseClock(lunchEnd)
		if startErr != nil || endErr != nil || lunchEndMinutes <= lunchStartMinutes {
			return 0
		}
		total -= lunchEndMinutes - lunchStartMinutes
	}
	return float64(total) / 60
}

func validateYearMonth(year, month int) error {
	if year < 1900 || year > 2200 || month < 1 || month > 12 {
		return fmt.Errorf("anno o mese non valido")
	}
	return nil
}

func neutralizeCSVFormula(value string) string {
	trimmed := strings.TrimLeft(value, " \t\r\n")
	if trimmed != "" && strings.ContainsRune("=+-@", rune(trimmed[0])) {
		return "'" + value
	}
	return value
}

func safeFilenamePart(value string) string {
	replacer := strings.NewReplacer("<", "_", ">", "_", ":", "_", "\"", "_", "/", "_", "\\", "_", "|", "_", "?", "_", "*", "_")
	clean := strings.Trim(replacer.Replace(value), " .")
	if clean == "" {
		return "Progetto"
	}
	return clean
}
