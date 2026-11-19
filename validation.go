package main

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"work-time-tracker-go/tracker"
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

// validateNewActivityReference rifiuta anche i tipi archiviati: restano validi per lo
// storico ma non si assegnano a nuove sessioni.
func (a *App) validateNewActivityReference(activityType *string) error {
	if err := a.validateActivityReference(activityType); err != nil || activityType == nil {
		return err
	}
	var archived int
	if err := a.db.QueryRow("SELECT COALESCE(archived, 0) FROM activity_types WHERE name = ?", strings.TrimSpace(*activityType)).Scan(&archived); err != nil {
		return err
	}
	if archived == 1 {
		return fmt.Errorf("il tipo attività %q è archiviato e non può essere usato per nuove sessioni", *activityType)
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
	return tracker.ParseLocalTimestamp(value)
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
