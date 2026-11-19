package tracker

import (
	"database/sql"
	"fmt"
	"math"
	"sort"
	"time"
)

// === RESOCONTO MENSILE ===

// DayEvent rappresenta un evento associato a un giorno (festa, ferie, permesso, malattia, trasferta)
type DayEvent struct {
	ID        int     `json:"id"`
	Date      string  `json:"date"`
	EventType string  `json:"event_type"` // "holiday", "vacation", "permesso", "malattia", "trasferta"
	Hours     float64 `json:"hours"`      // ore di permesso (0 per gli altri tipi)
}

// WorkScheduleConfig contiene l'orario di lavoro configurato (in minuti dalla mezzanotte)
type WorkScheduleConfig struct {
	StartMinutes      int // es. 480 per 08:00
	EndMinutes        int // es. 1020 per 17:00
	LunchBreakEnabled bool
	LunchStartMinutes int // es. 780 per 13:00
	LunchEndMinutes   int // es. 840 per 14:00
}

// DailyStats rappresenta le ore tracciate in un singolo giorno
type DailyStats struct {
	Date                 string  `json:"date"`
	DayOfWeek            string  `json:"day_of_week"`
	IsWeekend            bool    `json:"is_weekend"`
	TotalHours           float64 `json:"total_hours"`            // ore totali tracciate
	OutsideWorkHours     float64 `json:"outside_work_hours"`     // ore tracciate fuori orario (straordinari da tracciamento)
	EventType            string  `json:"event_type"`             // "", "holiday", "vacation", "permesso", "malattia", "trasferta"
	EventHours           float64 `json:"event_hours"`            // ore di permesso
	EffectiveWorkedHours float64 `json:"effective_worked_hours"` // ore lavorate effettive
	UsesDefault          bool    `json:"uses_default"`           // true se parte delle ore è assunta (non tracciata)
}

// ProjectStats rappresenta le ore tracciate per un progetto nel mese
type ProjectStats struct {
	ProjectName string  `json:"project_name"`
	TotalHours  float64 `json:"total_hours"`
}

// MonthlyReport contiene tutti i dati del resoconto mensile
type MonthlyReport struct {
	Year                      int            `json:"year"`
	Month                     int            `json:"month"`
	MonthName                 string         `json:"month_name"`
	ContractedHoursPerDay     float64        `json:"contracted_hours_per_day"`
	WorkingDaysInMonth        int            `json:"working_days_in_month"`
	TotalContractedHours      float64        `json:"total_contracted_hours"`
	EventsCoveredHours        float64        `json:"events_covered_hours"` // compatibilità: coincide con la riduzione per festività
	EffectiveContractedHours  float64        `json:"effective_contracted_hours"`
	HolidayDays               int            `json:"holiday_days"`
	HolidayHours              float64        `json:"holiday_hours"`
	VacationDays              int            `json:"vacation_days"`
	VacationHours             float64        `json:"vacation_hours"`
	SickDays                  int            `json:"sick_days"`
	SickHours                 float64        `json:"sick_hours"`
	TrasfertaDays             int            `json:"trasferta_days"`
	PermissionDays            float64        `json:"permission_days"`
	PermissionHours           float64        `json:"permission_hours"`
	TotalTrackedHours         float64        `json:"total_tracked_hours"`
	TotalEffectiveWorkedHours float64        `json:"total_effective_worked_hours"` // ore totali con default giorni non tracciati
	OvertimeHours             float64        `json:"overtime_hours"`
	Days                      []DailyStats   `json:"days"`
	Projects                  []ProjectStats `json:"projects"`
}

// === FUNZIONI PER EVENTI GIORNALIERI ===

// SetDayEvent imposta (o sovrascrive) un evento per un giorno specifico
func SetDayEvent(db *sql.DB, date, eventType string, hours float64) error {
	upsertSQL := `
	INSERT INTO day_events (date, event_type, hours)
	VALUES (?, ?, ?)
	ON CONFLICT(date) DO UPDATE SET event_type = excluded.event_type, hours = excluded.hours
	`
	_, err := db.Exec(upsertSQL, date, eventType, hours)
	if err != nil {
		return fmt.Errorf("errore salvataggio evento: %v", err)
	}
	fmt.Printf("[DB] Evento '%s' impostato per %s (%.2fh)\n", eventType, date, hours)
	return nil
}

// PopulateHolidays inserisce come 'holiday' le date fornite, senza sovrascrivere
// eventi già presenti su quei giorni. Ritorna quante date sono state inserite.
func PopulateHolidays(db *sql.DB, dates []string) (int, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("errore avvio inserimento festività: %v", err)
	}
	defer tx.Rollback()

	inserted := 0
	for _, date := range dates {
		result, err := tx.Exec(`INSERT INTO day_events (date, event_type, hours) VALUES (?, 'holiday', 0) ON CONFLICT(date) DO NOTHING`, date)
		if err != nil {
			return 0, fmt.Errorf("errore inserimento festività %s: %v", date, err)
		}
		if affected, _ := result.RowsAffected(); affected > 0 {
			inserted++
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("errore conferma inserimento festività: %v", err)
	}
	return inserted, nil
}

// GetYearHolidays restituisce le date (YYYY-MM-DD) marcate come festività
// nell'anno indicato, ordinate.
func GetYearHolidays(db *sql.DB, year int) ([]string, error) {
	prefix := fmt.Sprintf("%04d-", year)
	rows, err := db.Query(`SELECT date FROM day_events WHERE event_type = 'holiday' AND date LIKE ? ORDER BY date ASC`, prefix+"%")
	if err != nil {
		return nil, fmt.Errorf("errore query festività anno: %v", err)
	}
	defer rows.Close()

	var dates []string
	for rows.Next() {
		var date string
		if err := rows.Scan(&date); err != nil {
			return nil, err
		}
		dates = append(dates, date)
	}
	return dates, nil
}

// DeleteDayEvent rimuove l'evento associato a un giorno
func DeleteDayEvent(db *sql.DB, date string) error {
	_, err := db.Exec(`DELETE FROM day_events WHERE date = ?`, date)
	if err != nil {
		return fmt.Errorf("errore eliminazione evento: %v", err)
	}
	fmt.Printf("[DB] Evento rimosso per %s\n", date)
	return nil
}

// GetMonthDayEvents restituisce tutti gli eventi del mese specificato
func GetMonthDayEvents(db *sql.DB, year, month int) (map[string]DayEvent, error) {
	prefix := fmt.Sprintf("%04d-%02d-", year, month)
	rows, err := db.Query(`SELECT id, date, event_type, hours FROM day_events WHERE date LIKE ?`, prefix+"%")
	if err != nil {
		return nil, fmt.Errorf("errore query eventi: %v", err)
	}
	defer rows.Close()

	events := make(map[string]DayEvent)
	for rows.Next() {
		var e DayEvent
		if err := rows.Scan(&e.ID, &e.Date, &e.EventType, &e.Hours); err != nil {
			return nil, err
		}
		events[e.Date] = e
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("errore iterazione eventi: %v", err)
	}
	return events, nil
}

var monthNames = []string{
	"Gennaio", "Febbraio", "Marzo", "Aprile", "Maggio", "Giugno",
	"Luglio", "Agosto", "Settembre", "Ottobre", "Novembre", "Dicembre",
}

var dayNames = []string{"Domenica", "Lunedì", "Martedì", "Mercoledì", "Giovedì", "Venerdì", "Sabato"}

// maxSessionDurationDays è la durata massima di una sessione (vedi il limite
// di 366*24*60*60 secondi applicato in CreateSession). Usato per delimitare
// lo scan delle sessioni rilevanti per un mese senza escludere sessioni lunghe.
const maxSessionDurationDays = 366

// WorkingDaysBitmask è una maschera di bit che indica quali giorni sono lavorativi.
// Bit 0 = Domenica, Bit 1 = Lunedì, ..., Bit 6 = Sabato (corrisponde a time.Weekday).
// Default lunedì-venerdì = 0b0111110 = 62.
const DefaultWorkingDaysBitmask = 62

// IsWorkingDay verifica se il giorno della settimana è lavorativo secondo la bitmask.
func IsWorkingDay(weekday time.Weekday, bitmask int) bool {
	return (bitmask>>uint(weekday))&1 == 1
}

// intervalOverlap restituisce i secondi di sovrapposizione tra [s1,e1] e [s2,e2]
func intervalOverlap(s1, e1, s2, e2 time.Time) int64 {
	start := s1
	if s2.After(start) {
		start = s2
	}
	end := e1
	if e2.Before(end) {
		end = e2
	}
	if !end.After(start) {
		return 0
	}
	return int64(end.Sub(start).Seconds())
}

// outsideWorkSeconds calcola i secondi di [sessStart, sessEnd] che cadono fuori dall'orario lavorativo.
// Con pausa pranzo abilitata, la finestra effettiva è [workStart,lunchStart] + [lunchEnd,workEnd]:
// il tracciamento durante la pausa conta come straordinario.
func outsideWorkSeconds(sessStart, sessEnd time.Time, cfg WorkScheduleConfig) int64 {
	total := int64(sessEnd.Sub(sessStart).Seconds())
	if total <= 0 {
		return 0
	}
	wStart := time.Date(sessStart.Year(), sessStart.Month(), sessStart.Day(),
		cfg.StartMinutes/60, cfg.StartMinutes%60, 0, 0, sessStart.Location())
	wEnd := time.Date(sessStart.Year(), sessStart.Month(), sessStart.Day(),
		cfg.EndMinutes/60, cfg.EndMinutes%60, 0, 0, sessStart.Location())

	var inside int64
	if cfg.LunchBreakEnabled && cfg.LunchEndMinutes > cfg.LunchStartMinutes {
		lStart := time.Date(sessStart.Year(), sessStart.Month(), sessStart.Day(),
			cfg.LunchStartMinutes/60, cfg.LunchStartMinutes%60, 0, 0, sessStart.Location())
		lEnd := time.Date(sessStart.Year(), sessStart.Month(), sessStart.Day(),
			cfg.LunchEndMinutes/60, cfg.LunchEndMinutes%60, 0, 0, sessStart.Location())
		// Finestra effettiva: mattina [wStart, lStart] + pomeriggio [lEnd, wEnd]
		inside = intervalOverlap(sessStart, sessEnd, wStart, lStart) +
			intervalOverlap(sessStart, sessEnd, lEnd, wEnd)
	} else {
		inside = intervalOverlap(sessStart, sessEnd, wStart, wEnd)
	}

	outside := total - inside
	if outside < 0 {
		return 0
	}
	return outside
}

// GetMonthlyReport genera il resoconto mensile per anno e mese specificati
func GetMonthlyReport(db *sql.DB, year, month int, contractedHoursPerDay float64, workingDaysBitmask int, schedule WorkScheduleConfig) (*MonthlyReport, error) {
	if month < 1 || month > 12 || year < 1900 || year > 2200 {
		return nil, fmt.Errorf("anno o mese non valido")
	}
	if contractedHoursPerDay < 0 || contractedHoursPerDay > 24 {
		return nil, fmt.Errorf("ore contrattuali non valide")
	}

	monthStart := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Local)
	monthEnd := monthStart.AddDate(0, 1, 0)
	hasSchedule := schedule.EndMinutes > schedule.StartMinutes

	eventsMap, err := GetMonthDayEvents(db, year, month)
	if err != nil {
		return nil, err
	}

	// Seleziona tutte le sessioni che si sovrappongono al mese, comprese
	// quelle iniziate nel mese precedente o terminate nel successivo.
	// I primi due filtri confrontano direttamente la colonna timestamp (senza
	// wrapping in funzioni) cosi' sfruttano idx_sessions_timestamp; il limite
	// inferiore usa la durata massima di sessione ammessa dall'app (vedi
	// CreateSession) per restringere lo scan senza perdere sessioni lunghe che
	// si protraggono nel mese. Il controllo esatto sull'orario di fine resta
	// nell'ultima condizione, che necessita ancora dell'aritmetica su data/ora.
	lowerBound := monthStart.AddDate(0, 0, -maxSessionDurationDays)
	rows, err := db.Query(`
		SELECT s.seconds, s.timestamp, COALESCE(p.name, 'Nessun progetto')
		FROM sessions s
		LEFT JOIN projects p ON p.id = s.project_id
		WHERE s.seconds > 0
		  AND s.timestamp >= ?
		  AND s.timestamp < ?
		  AND datetime(replace(substr(s.timestamp, 1, 19), 'T', ' '), printf('+%d seconds', s.seconds)) > ?
	`, lowerBound.Format("2006-01-02 15:04:05"), monthEnd.Format("2006-01-02 15:04:05"), monthStart.Format("2006-01-02 15:04:05"))
	if err != nil {
		return nil, fmt.Errorf("errore query sessioni mensili: %v", err)
	}
	defer rows.Close()

	daySecondsMap := make(map[string]int64)
	outsideSecsByDay := make(map[string]int64)
	projectSeconds := make(map[string]int64)
	var totalTrackedSeconds int64

	for rows.Next() {
		var seconds int
		var timestamp, projectName string
		if err := rows.Scan(&seconds, &timestamp, &projectName); err != nil {
			return nil, fmt.Errorf("errore lettura sessione mensile: %v", err)
		}
		sessionStart, err := ParseLocalTimestamp(timestamp)
		if err != nil {
			return nil, err
		}
		sessionEnd := sessionStart.Add(time.Duration(seconds) * time.Second)
		if sessionStart.Before(monthStart) {
			sessionStart = monthStart
		}
		if sessionEnd.After(monthEnd) {
			sessionEnd = monthEnd
		}
		if !sessionEnd.After(sessionStart) {
			continue
		}

		clampedSeconds := int64(sessionEnd.Sub(sessionStart) / time.Second)
		projectSeconds[projectName] += clampedSeconds
		totalTrackedSeconds += clampedSeconds

		// Ogni segmento appartiene a un solo giorno; in questo modo le sessioni
		// a cavallo della mezzanotte vengono attribuite correttamente.
		segmentStart := sessionStart
		for segmentStart.Before(sessionEnd) {
			nextMidnight := time.Date(segmentStart.Year(), segmentStart.Month(), segmentStart.Day(), 0, 0, 0, 0, segmentStart.Location()).AddDate(0, 0, 1)
			segmentEnd := sessionEnd
			if nextMidnight.Before(segmentEnd) {
				segmentEnd = nextMidnight
			}
			segmentSeconds := int64(segmentEnd.Sub(segmentStart) / time.Second)
			date := segmentStart.Format("2006-01-02")
			daySecondsMap[date] += segmentSeconds

			workingDay := IsWorkingDay(segmentStart.Weekday(), workingDaysBitmask)
			event := eventsMap[date]
			if !workingDay || event.EventType == "holiday" || event.EventType == "vacation" || event.EventType == "malattia" {
				outsideSecsByDay[date] += segmentSeconds
			} else if hasSchedule {
				outsideSecsByDay[date] += outsideWorkSeconds(segmentStart, segmentEnd, schedule)
			}
			segmentStart = segmentEnd
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("errore iterazione sessioni mensili: %v", err)
	}

	// Calcola giorni lavorativi nel mese in base alla bitmask configurata
	workingDays := 0
	current := monthStart
	for current.Month() == time.Month(month) {
		if IsWorkingDay(current.Weekday(), workingDaysBitmask) {
			workingDays++
		}
		current = current.AddDate(0, 0, 1)
	}

	// Costruisci lista giorni del mese
	var days []DailyStats
	totalEffectiveWorkedHours := 0.0
	holidayHours := 0.0
	holidayDays := 0
	vacationHours := 0.0
	vacationDays := 0
	sickHours := 0.0
	sickDays := 0
	trasfertaDays := 0
	permissionHours := 0.0
	permissionDays := 0.0
	d := monthStart
	for d.Month() == time.Month(month) {
		dateStr := d.Format("2006-01-02")
		seconds := daySecondsMap[dateStr]
		isWeekend := !IsWorkingDay(d.Weekday(), workingDaysBitmask)

		eventType := ""
		eventHours := 0.0
		if ev, ok := eventsMap[dateStr]; ok {
			eventType = ev.EventType
			if ev.EventType == "permesso" {
				eventHours = math.Min(math.Max(ev.Hours, 0), contractedHoursPerDay)
				if !isWeekend {
					permissionHours += eventHours
					if contractedHoursPerDay > 0 {
						permissionDays += eventHours / contractedHoursPerDay
					}
				}
			} else if ev.EventType == "vacation" && !isWeekend {
				eventHours = contractedHoursPerDay
				vacationHours += contractedHoursPerDay
				vacationDays++
			} else if ev.EventType == "malattia" && !isWeekend {
				eventHours = contractedHoursPerDay
				sickHours += contractedHoursPerDay
				sickDays++
			} else if ev.EventType == "trasferta" && !isWeekend {
				// Puro promemoria: non tocca il calcolo delle ore, il giorno
				// resta lavorato normalmente (dovuto o straordinario).
				trasfertaDays++
			} else if ev.EventType == "holiday" && !isWeekend {
				eventHours = contractedHoursPerDay
				holidayHours += contractedHoursPerDay
				holidayDays++
			}
		}

		// Calcola ore lavorate effettive per questo giorno.
		// Regola: ore_effettive = ore_contratto + ore_tracciate_fuori_orario
		// Il tracciamento dentro l'orario indica solo su cosa si è lavorato (non cambia il totale).
		// Il tracciamento fuori orario è straordinario reale da sommare.
		var effectiveWorked float64
		var usesDefault bool
		trackedHours := float64(seconds) / 3600.0
		outsideHours := float64(outsideSecsByDay[dateStr]) / 3600.0

		if isWeekend {
			// Weekend: conta solo il tracciato (nessun floor contrattuale)
			effectiveWorked = trackedHours
		} else if eventType == "holiday" {
			// Festività: riduce il monte contrattuale; eventuali ore tracciate sono straordinario.
			effectiveWorked = outsideHours
		} else if eventType == "vacation" || eventType == "permesso" || eventType == "malattia" {
			// Ferie, permessi e malattia sono assenze giustificate: non riducono le ore
			// contrattuali e coprono il normale monte giornaliero. Solo il lavoro fuori orario si somma.
			effectiveWorked = contractedHoursPerDay + outsideHours
			usesDefault = true
		} else if hasSchedule {
			// Orario configurato: contracted + straordinari fuori finestra
			effectiveWorked = contractedHoursPerDay + outsideHours
			usesDefault = trackedHours-outsideHours < contractedHoursPerDay
		} else {
			// Nessun orario configurato: fallback max(contracted, tracked)
			effectiveWorked = math.Max(contractedHoursPerDay, trackedHours)
			usesDefault = trackedHours < contractedHoursPerDay
		}
		totalEffectiveWorkedHours += effectiveWorked

		days = append(days, DailyStats{
			Date:                 dateStr,
			DayOfWeek:            dayNames[d.Weekday()],
			IsWeekend:            isWeekend,
			TotalHours:           trackedHours,
			OutsideWorkHours:     outsideHours,
			EventType:            eventType,
			EventHours:           eventHours,
			EffectiveWorkedHours: effectiveWorked,
			UsesDefault:          usesDefault,
		})
		d = d.AddDate(0, 0, 1)
	}

	var projects []ProjectStats
	for projectName, seconds := range projectSeconds {
		projects = append(projects, ProjectStats{ProjectName: projectName, TotalHours: float64(seconds) / 3600.0})
	}
	sort.Slice(projects, func(i, j int) bool { return projects[i].TotalHours > projects[j].TotalHours })

	totalTrackedHours := float64(totalTrackedSeconds) / 3600.0
	totalContractedHours := float64(workingDays) * contractedHoursPerDay
	effectiveContractedHours := totalContractedHours - holidayHours
	if effectiveContractedHours < 0 {
		effectiveContractedHours = 0
	}
	overtimeHours := totalEffectiveWorkedHours - effectiveContractedHours

	monthName := ""
	if month >= 1 && month <= 12 {
		monthName = fmt.Sprintf("%s %d", monthNames[month-1], year)
	}

	return &MonthlyReport{
		Year:                      year,
		Month:                     month,
		MonthName:                 monthName,
		ContractedHoursPerDay:     contractedHoursPerDay,
		WorkingDaysInMonth:        workingDays,
		TotalContractedHours:      totalContractedHours,
		EventsCoveredHours:        holidayHours,
		EffectiveContractedHours:  effectiveContractedHours,
		HolidayDays:               holidayDays,
		HolidayHours:              holidayHours,
		VacationDays:              vacationDays,
		VacationHours:             vacationHours,
		SickDays:                  sickDays,
		SickHours:                 sickHours,
		TrasfertaDays:             trasfertaDays,
		PermissionDays:            permissionDays,
		PermissionHours:           permissionHours,
		TotalTrackedHours:         totalTrackedHours,
		TotalEffectiveWorkedHours: totalEffectiveWorkedHours,
		OvertimeHours:             overtimeHours,
		Days:                      days,
		Projects:                  projects,
	}, nil
}
